use std::sync::Arc;
use tracing::info;
use opcua::types::Variant;

use crate::listeners::{NodeData, OpcuaService};
use crate::models::*;

/// Manager de suscripciones OPC UA
pub struct SubscriptionManager {
    opcua_service: Arc<OpcuaService>,
}

impl SubscriptionManager {
    pub fn new(opcua_service: Arc<OpcuaService>) -> Self {
        Self { opcua_service }
    }

    /// Configurar suscripción a vectores WAGO
    pub async fn setup_wago_vector_subscription(&self) -> anyhow::Result<()> {
        let service = self.opcua_service.clone();

        let node_ids = vec![WAGO_VECTOR_BOOL, WAGO_VECTOR_INT, WAGO_VECTOR_WORD];

        self.opcua_service
            .subscribe_to_nodes(
                WAGO_VECTOR_SUBSCRIPTION,
                node_ids,
                move |node_id, node_data| {
                    let service = service.clone();
                    tokio::spawn(async move {
                        Self::handle_wago_vector_data(service, node_id, node_data).await;
                    });
                },
            )
            .await?;

        info!("✅ Suscripción a vectores WAGO configurada");
        Ok(())
    }

    /// Manejar datos de vectores WAGO
    async fn handle_wago_vector_data(
        service: Arc<OpcuaService>,
        node_id: String,
        node_data: NodeData,
    ) {
        if node_id.contains("VectorBool") {
            if let Variant::Array(ref arr) = node_data.value {
                if let Some(Variant::Boolean(first_val)) = arr.values.first() {
                    let _ = service
                        .queue_write(WAGO_BOLEANO_TEST.to_string(), Variant::Boolean(!first_val))
                        .await;
                }
            }
        } else if node_id.contains("VectorInt") {
            if let Variant::Array(ref arr) = node_data.value {
                if let Some(Variant::Int16(first_val)) = arr.values.first() {
                    let new_value = first_val.wrapping_add(1);
                    let _ = service
                        .queue_write(WAGO_ENTERO_TEST.to_string(), Variant::Int16(new_value))
                        .await;
                }
            }
        } else if node_id.contains("VectorWord") {
            if let Variant::Array(ref arr) = node_data.value {
                if let Some(Variant::UInt16(first_val)) = arr.values.first() {
                    let new_word = first_val.wrapping_add(10);
                    let _ = service
                        .queue_write(WAGO_WORD_TEST.to_string(), Variant::UInt16(new_word))
                        .await;

                    let new_string = format!("Word_{}", first_val);
                    let _ = service
                        .queue_write(
                            WAGO_STRING_TEST.to_string(),
                            Variant::String(new_string.into()),
                        )
                        .await;
                }
            }
        }
    }

    /// Configurar suscripciones por defecto
    pub async fn setup_default_subscriptions(&self) -> anyhow::Result<()> {
        let node_ids = vec![DEFAULT_SEGREGATION_NODE];

        self.opcua_service
            .subscribe_to_nodes(DEFAULT_SUBSCRIPTION, node_ids, |node_id, node_data| {
                info!("🔄 Dato recibido | Nodo: {} | Valor: {:?}", node_id, node_data.value);
            })
            .await?;

        info!("✅ Suscripciones por defecto configuradas");
        Ok(())
    }
}
