mod flow;
mod http;
mod listeners;
mod models;

use anyhow::Result;
use std::env;
use std::sync::Arc;
use tokio::task;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

use crate::flow::{wago_loop, SubscriptionManager};
use crate::http::{create_router, AppState};
use crate::listeners::OpcuaService;

#[tokio::main]
async fn main() -> Result<()> {
    // Cargar variables de entorno
    dotenv::dotenv().ok();

    // Configurar logger
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    info!("🚀 Iniciando API-Greenex-Rust...");

    // Obtener endpoint OPC UA desde variable de entorno
    let opcua_endpoint = env::var("OPCUA_ENDPOINT")
        .unwrap_or_else(|_| "opc.tcp://localhost:4840".to_string());

    info!("📡 Endpoint OPC UA: {}", opcua_endpoint);

    // Crear servicio OPC UA
    let opcua_service = Arc::new(OpcuaService::new(opcua_endpoint));

    // Conectar a OPC UA
    match opcua_service.connect().await {
        Ok(_) => info!("✅ Conectado a OPC UA exitosamente"),
        Err(e) => {
            error!("❌ Error conectando a OPC UA: {}", e);
            info!("⚠️  Continuando en modo desconectado...");
        }
    }

    // Crear subscription manager
    let subscription_manager = SubscriptionManager::new(opcua_service.clone());

    // Configurar suscripciones
    if opcua_service.is_connected().await {
        if let Err(e) = subscription_manager.setup_wago_vector_subscription().await {
            error!("❌ Error configurando suscripción WAGO: {}", e);
        }

        if let Err(e) = subscription_manager.setup_default_subscriptions().await {
            error!("❌ Error configurando suscripciones por defecto: {}", e);
        }
    }

    // Iniciar procesador de escrituras en background
    let opcua_service_writer = opcua_service.clone();
    task::spawn(async move {
        opcua_service_writer.process_writes().await;
    });

    // Iniciar keep-alive para reconexión automática
    let opcua_service_keepalive = opcua_service.clone();
    task::spawn(async move {
        opcua_service_keepalive.keep_alive().await;
    });

    // Iniciar WAGO loop
    let opcua_service_wago = opcua_service.clone();
    task::spawn(async move {
        wago_loop(opcua_service_wago).await;
    });

    // Configurar servidor HTTP
    let http_port = env::var("HTTP_PORT").unwrap_or_else(|_| "8080".to_string());
    let addr = format!("0.0.0.0:{}", http_port);

    let app_state = AppState {
        opcua_service: opcua_service.clone(),
    };

    let app = create_router(app_state);

    info!("🌐 Servidor HTTP iniciando en {}", addr);

    // Iniciar servidor HTTP
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
