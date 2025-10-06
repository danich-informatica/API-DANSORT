use axum::{
    extract::State,
    http::StatusCode,
    response::{Html, IntoResponse, Json},
    routing::{get, post},
    Router,
};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::sync::Arc;
use tracing::info;

use crate::listeners::OpcuaService;
use crate::models::*;
use opcua::types::Variant;

/// Estado compartido de la aplicación
#[derive(Clone)]
pub struct AppState {
    pub opcua_service: Arc<OpcuaService>,
}

/// Respuesta de estado del sistema
#[derive(Serialize)]
pub struct StatusResponse {
    status: String,
    service: String,
    opcua: OpcuaStatus,
}

#[derive(Serialize)]
pub struct OpcuaStatus {
    connected: bool,
}

/// Request para escribir en nodo WAGO
#[derive(Deserialize)]
pub struct WriteWagoRequest {
    value: serde_json::Value,
}

/// Crear router HTTP
pub fn create_router(state: AppState) -> Router {
    Router::new()
        .route("/", get(root_handler))
        .route("/status", get(status_handler))
        .route("/wago/:variable", post(write_wago_handler))
        .with_state(state)
}

/// Handler raíz
async fn root_handler() -> impl IntoResponse {
    Html("<h1>API-Greenex-Rust</h1><p>OPC UA Service Running</p>")
}

/// Handler de estado (JSON)
async fn status_handler(State(state): State<AppState>) -> impl IntoResponse {
    let connected = state.opcua_service.is_connected().await;

    let response = StatusResponse {
        status: "OK".to_string(),
        service: "API-Greenex-Rust".to_string(),
        opcua: OpcuaStatus { connected },
    };

    Json(response)
}

/// Handler para escribir en variables WAGO
async fn write_wago_handler(
    State(state): State<AppState>,
    axum::extract::Path(variable): axum::extract::Path<String>,
    Json(payload): Json<WriteWagoRequest>,
) -> impl IntoResponse {
    // Mapear nombre de variable a NodeID
    let node_id = match variable.as_str() {
        "boleano" => WAGO_BOLEANO_TEST,
        "byte" => WAGO_BYTE_TEST,
        "entero" => WAGO_ENTERO_TEST,
        "real" => WAGO_REAL_TEST,
        "string" => WAGO_STRING_TEST,
        "word" => WAGO_WORD_TEST,
        _ => {
            return (
                StatusCode::BAD_REQUEST,
                Json(json!({
                    "error": format!("Variable desconocida: {}", variable)
                })),
            );
        }
    };

    // Convertir valor JSON a Variant OPC UA
    let variant = match variable.as_str() {
        "boleano" => {
            if let Some(b) = payload.value.as_bool() {
                Variant::Boolean(b)
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(json!({ "error": "Valor debe ser booleano" })),
                );
            }
        }
        "byte" => {
            if let Some(n) = payload.value.as_u64() {
                if n <= 255 {
                    Variant::Byte(n as u8)
                } else {
                    return (
                        StatusCode::BAD_REQUEST,
                        Json(json!({ "error": "Byte debe estar entre 0-255" })),
                    );
                }
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(json!({ "error": "Valor debe ser número" })),
                );
            }
        }
        "entero" => {
            if let Some(n) = payload.value.as_i64() {
                Variant::Int16(n as i16)
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(json!({ "error": "Valor debe ser entero" })),
                );
            }
        }
        "real" => {
            if let Some(f) = payload.value.as_f64() {
                Variant::Float(f as f32)
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(json!({ "error": "Valor debe ser número" })),
                );
            }
        }
        "string" => {
            if let Some(s) = payload.value.as_str() {
                Variant::String(s.into())
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(json!({ "error": "Valor debe ser string" })),
                );
            }
        }
        "word" => {
            if let Some(n) = payload.value.as_u64() {
                if n <= 65535 {
                    Variant::UInt16(n as u16)
                } else {
                    return (
                        StatusCode::BAD_REQUEST,
                        Json(json!({ "error": "Word debe estar entre 0-65535" })),
                    );
                }
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(json!({ "error": "Valor debe ser número" })),
                );
            }
        }
        _ => unreachable!(),
    };

    // Escribir valor
    match state.opcua_service.write_node(node_id, variant).await {
        Ok(_) => (
            StatusCode::OK,
            Json(json!({
                "status": "success",
                "message": format!("Valor escrito en {}", variable)
            })),
        ),
        Err(e) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(json!({
                "error": format!("Error escribiendo en {}: {}", variable, e)
            })),
        ),
    }
}
