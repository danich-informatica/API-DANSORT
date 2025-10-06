use anyhow::{Context, Result};
use async_channel::{Receiver, Sender};
use chrono::Utc;
use opcua::client::prelude::*;
use opcua::types::{DataValue, NodeId, StatusCode, Variant};
use std::sync::Arc;
use tokio::sync::RwLock;
use tokio::time::{sleep, Duration};
use tracing::{debug, error, info, warn};

use crate::models::*;

/// Datos de un nodo OPC UA
#[derive(Debug, Clone)]
pub struct NodeData {
    pub node_id: String,
    pub value: Variant,
    pub timestamp: DateTime,
    pub quality: StatusCode,
}

/// Solicitud de escritura
#[derive(Debug, Clone)]
pub struct WriteRequest {
    pub node_id: String,
    pub value: Variant,
}

/// Estado de la conexión
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum ConnectionState {
    Connected,
    Disconnected,
    Connecting,
}

/// Servicio OPC UA
pub struct OpcuaService {
    client: Arc<RwLock<Option<Client>>>,
    session: Arc<RwLock<Option<Arc<RwLock<Session>>>>>,
    endpoint: String,
    state: Arc<RwLock<ConnectionState>>,
    write_tx: Sender<WriteRequest>,
    write_rx: Receiver<WriteRequest>,
}

impl OpcuaService {
    pub fn new(endpoint: String) -> Self {
        let (write_tx, write_rx) = async_channel::bounded(50);

        Self {
            client: Arc::new(RwLock::new(None)),
            session: Arc::new(RwLock::new(None)),
            endpoint,
            state: Arc::new(RwLock::new(ConnectionState::Disconnected)),
            write_tx,
            write_rx,
        }
    }

    /// Conectar al servidor OPC UA
    pub async fn connect(&self) -> Result<()> {
        info!("🔌 Conectando a OPC UA: {}", self.endpoint);
        *self.state.write().await = ConnectionState::Connecting;

        // Crear cliente OPC UA
        let mut client_builder = ClientBuilder::new()
            .application_name("API-Greenex-Rust")
            .application_uri("urn:API-Greenex-Rust")
            .trust_server_certs(true)
            .create_sample_keypair(true)
            .session_retry_limit(3);

        let client = client_builder
            .client()
            .context("Error creando cliente OPC UA")?;

        // Crear sesión
        let session = client
            .connect_to_endpoint(
                (
                    self.endpoint.as_str(),
                    SecurityPolicy::None.to_str(),
                    MessageSecurityMode::None,
                    UserTokenPolicy::anonymous(),
                ),
                IdentityToken::Anonymous,
            )
            .await
            .context("Error conectando al endpoint OPC UA")?;

        *self.client.write().await = Some(client);
        *self.session.write().await = Some(session);
        *self.state.write().await = ConnectionState::Connected;

        info!("✅ Conectado a OPC UA exitosamente");
        Ok(())
    }

    /// Desconectar del servidor
    pub async fn disconnect(&self) -> Result<()> {
        info!("🔌 Desconectando de OPC UA...");

        if let Some(session) = self.session.write().await.take() {
            let session_lock = session.read().await;
            session_lock
                .disconnect()
                .await
                .context("Error desconectando sesión")?;
        }

        *self.client.write().await = None;
        *self.state.write().await = ConnectionState::Disconnected;

        info!("✅ Desconectado de OPC UA");
        Ok(())
    }

    /// Verificar si está conectado
    pub async fn is_connected(&self) -> bool {
        *self.state.read().await == ConnectionState::Connected
    }

    /// Leer un nodo
    pub async fn read_node(&self, node_id: &str) -> Result<NodeData> {
        let start = std::time::Instant::now();

        if !self.is_connected().await {
            return Err(anyhow::anyhow!("Cliente no está conectado"));
        }

        let session_lock = self
            .session
            .read()
            .await
            .clone()
            .context("Sesión no disponible")?;

        let session = session_lock.read().await;

        let node_id_parsed = NodeId::parse(node_id)
            .map_err(|e| anyhow::anyhow!("Error parseando NodeID {}: {}", node_id, e))?;

        let read_result = session
            .read(&[node_id_parsed.clone()], TimestampsToReturn::Both, 1.0)
            .await
            .context("Error leyendo nodo")?;

        if read_result.is_empty() {
            return Err(anyhow::anyhow!("Respuesta vacía al leer nodo {}", node_id));
        }

        let data_value = &read_result[0];

        if data_value.status.is_bad() {
            return Err(anyhow::anyhow!(
                "Error en lectura del nodo {}: {:?}",
                node_id,
                data_value.status
            ));
        }

        let value = data_value
            .value
            .clone()
            .unwrap_or_else(|| Variant::Empty);

        debug!(
            "✅ Lectura exitosa | Nodo: {} | Duración: {:?}",
            node_id,
            start.elapsed()
        );

        Ok(NodeData {
            node_id: node_id.to_string(),
            value,
            timestamp: data_value.server_timestamp.unwrap_or_else(DateTime::now),
            quality: data_value.status.clone().unwrap(),
        })
    }

    /// Escribir en un nodo con logs detallados
    pub async fn write_node(&self, node_id: &str, value: Variant) -> Result<()> {
        let start = std::time::Instant::now();

        if !self.is_connected().await {
            error!("❌ WRITE ERROR | Nodo: {} | ERROR: Cliente no conectado", node_id);
            return Err(anyhow::anyhow!("Cliente no está conectado"));
        }

        let session_lock = self
            .session
            .read()
            .await
            .clone()
            .context("Sesión no disponible")?;

        let session = session_lock.read().await;

        let node_id_parsed = NodeId::parse(node_id)
            .map_err(|e| anyhow::anyhow!("Error parseando NodeID {}: {}", node_id, e))?;

        // Leer el DataType esperado del nodo
        let data_type_info = match self.read_node_data_type(node_id).await {
            Ok(dt) => format!("{:?}", dt),
            Err(_) => "Desconocido".to_string(),
        };

        info!("┌─────────────────────────────────────────────────────────");
        info!("│ 📝 ESCRITURA WAGO");
        info!("│ Nodo: {}", node_id);
        info!("│ Tipo esperado (DataType): {}", data_type_info);
        info!("│ Valor a escribir: {:?}", value);
        info!("│ Tipo Variant: {:?}", value.type_id());
        info!("└─────────────────────────────────────────────────────────");

        let data_value = DataValue {
            value: Some(value.clone()),
            status: Some(StatusCode::Good),
            server_timestamp: Some(DateTime::now()),
            source_timestamp: Some(DateTime::now()),
            server_picoseconds: None,
            source_picoseconds: None,
        };

        let write_result = session
            .write(&[(node_id_parsed.clone(), data_value)])
            .await
            .context("Error escribiendo en nodo")?;

        let duration = start.elapsed();

        if write_result.is_empty() {
            error!("❌ WRITE FAILED | Nodo: {} | ERROR: Respuesta vacía", node_id);
            return Err(anyhow::anyhow!("Respuesta vacía al escribir en nodo"));
        }

        let status = &write_result[0];

        if status.is_bad() {
            error!("┌─────────────────────────────────────────────────────────");
            error!("│ ❌ WRITE STATUS ERROR");
            error!("│ Nodo: {}", node_id);
            error!("│ Valor enviado: {:?}", value);
            error!("│ Tipo Variant: {:?}", value.type_id());
            error!("│ DataType esperado: {}", data_type_info);
            error!("│ Status Code: {:?}", status);
            error!("│ Duración: {:?}", duration);
            error!("└─────────────────────────────────────────────────────────");

            return Err(anyhow::anyhow!(
                "Error en escritura del nodo {}: {:?}",
                node_id,
                status
            ));
        }

        info!("✅ ÉXITO | Nodo: {} | Valor: {:?} | Duración: {:?}", node_id, value, duration);
        Ok(())
    }

    /// Leer el DataType de un nodo
    async fn read_node_data_type(&self, node_id: &str) -> Result<NodeId> {
        if !self.is_connected().await {
            return Err(anyhow::anyhow!("Cliente no está conectado"));
        }

        let session_lock = self
            .session
            .read()
            .await
            .clone()
            .context("Sesión no disponible")?;

        let session = session_lock.read().await;

        let node_id_parsed = NodeId::parse(node_id)
            .map_err(|e| anyhow::anyhow!("Error parseando NodeID {}: {}", node_id, e))?;

        let attributes = session
            .read_node_attributes(&node_id_parsed)
            .await
            .context("Error leyendo atributos del nodo")?;

        attributes
            .data_type
            .ok_or_else(|| anyhow::anyhow!("DataType no disponible para el nodo"))
    }

    /// Encolar solicitud de escritura
    pub async fn queue_write(&self, node_id: String, value: Variant) -> Result<()> {
        self.write_tx
            .send(WriteRequest { node_id, value })
            .await
            .context("Error encolando escritura")
    }

    /// Procesar escrituras en background
    pub async fn process_writes(&self) {
        info!("🔄 Procesador de escrituras OPC UA iniciado");

        while let Ok(req) = self.write_rx.recv().await {
            if !self.is_connected().await {
                warn!("Descartando escritura en {}: cliente no conectado", req.node_id);
                continue;
            }

            if let Err(e) = self.write_node(&req.node_id, req.value).await {
                error!("Error procesando escritura: {}", e);
            }
        }

        info!("🔄 Procesador de escrituras OPC UA detenido");
    }

    /// Crear suscripción a nodos con callback
    pub async fn subscribe_to_nodes<F>(
        &self,
        subscription_name: &str,
        node_ids: Vec<&str>,
        callback: F,
    ) -> Result<()>
    where
        F: Fn(String, NodeData) + Send + Sync + 'static,
    {
        if !self.is_connected().await {
            return Err(anyhow::anyhow!("Cliente no está conectado"));
        }

        let session_lock = self
            .session
            .read()
            .await
            .clone()
            .context("Sesión no disponible")?;

        let session = session_lock.read().await;

        let items: Vec<_> = node_ids
            .iter()
            .filter_map(|node_id| {
                NodeId::parse(node_id)
                    .ok()
                    .map(|nid| nid.into())
            })
            .collect();

        if items.is_empty() {
            return Err(anyhow::anyhow!("No hay nodos válidos para suscribir"));
        }

        let subscription_id = session
            .create_subscription(
                Duration::from_millis(500),
                10,
                30,
                0,
                0,
                true,
                DataChangeCallback::new(move |changed_items| {
                    for item in changed_items {
                        if let Some(ref value) = item.value().value {
                            let node_data = NodeData {
                                node_id: format!("{:?}", item.item_to_monitor().node_id),
                                value: value.clone(),
                                timestamp: item.value().server_timestamp.unwrap_or_else(DateTime::now),
                                quality: item.value().status.clone().unwrap(),
                            };
                            callback(node_data.node_id.clone(), node_data);
                        }
                    }
                }),
            )
            .await
            .context("Error creando suscripción")?;

        session
            .create_monitored_items(subscription_id, TimestampsToReturn::Both, &items)
            .await
            .context("Error creando elementos monitoreados")?;

        info!(
            "✅ Suscripción '{}' creada para {} nodos",
            subscription_name,
            node_ids.len()
        );

        Ok(())
    }

    /// Mantener conexión activa con reconexión automática
    pub async fn keep_alive(&self) {
        loop {
            sleep(Duration::from_secs(5)).await;

            if !self.is_connected().await {
                warn!("⚠️  Conexión perdida. Intentando reconectar...");
                if let Err(e) = self.connect().await {
                    error!("Error en reconexión: {}", e);
                } else {
                    info!("✅ Reconexión exitosa");
                }
            }
        }
    }
}
