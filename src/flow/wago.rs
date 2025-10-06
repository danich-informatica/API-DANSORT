use std::sync::Arc;
use tokio::time::{sleep, Duration};
use tracing::{error, info};
use opcua::types::Variant;

use crate::listeners::OpcuaService;
use crate::models::*;

/// Intervalo del bucle WAGO (10 segundos)
const WAGO_LOOP_INTERVAL: Duration = Duration::from_secs(10);

/// Bucle principal de escritura y lectura de nodos WAGO
pub async fn wago_loop(service: Arc<OpcuaService>) {
    info!("🔄 Iniciando WAGO Loop...");

    loop {
        if !service.is_connected().await {
            sleep(Duration::from_secs(1)).await;
            continue;
        }

        // Generar valores de prueba
        let timestamp = chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0);
        let boleano = (timestamp % 2) == 0;
        let byte_val = (timestamp % 256) as u8;
        let entero = (timestamp % 32768) as i16;
        let real_val = ((timestamp % 10000) as f32) / 100.0;
        let str_val = format!("LoopTest-{}", timestamp % 1000);
        let word_val = (timestamp % 65536) as u16;

        // Operaciones de escritura
        let write_ops = vec![
            (WAGO_BOLEANO_TEST, Variant::Boolean(boleano)),
            (WAGO_BYTE_TEST, Variant::Byte(byte_val)),
            (WAGO_ENTERO_TEST, Variant::Int16(entero)),
            (WAGO_REAL_TEST, Variant::Float(real_val)),
            (WAGO_STRING_TEST, Variant::String(str_val.into())),
            (WAGO_WORD_TEST, Variant::UInt16(word_val)),
        ];

        for (node_id, value) in write_ops {
            // Leer DataType esperado (opcional, para debugging)
            if let Err(e) = service.read_node_data_type(node_id).await {
                info!("⚠️  Error leyendo DataType de {}: {}", node_id, e);
            }

            // Escribir valor
            if let Err(e) = service.write_node(node_id, value.clone()).await {
                error!("❌ FALLO | Nodo: {} | Valor: {:?} | Error: {}", node_id, value, e);
            } else {
                info!("✅ ÉXITO | Nodo: {} | Valor: {:?}", node_id, value);
            }
        }

        // Lectura de vectores (sin logs, solo para mantener estado)
        let vector_nodes = vec![WAGO_VECTOR_BOOL, WAGO_VECTOR_INT, WAGO_VECTOR_WORD];
        
        for node_id in vector_nodes {
            let _ = service.read_node(node_id).await;
        }

        sleep(WAGO_LOOP_INTERVAL).await;
    }
}
