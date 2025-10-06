use std::fmt;

/// Constantes OPC UA
pub const OPCUA_NAMESPACE: u16 = 4;
pub const OPCUA_SEGREGATION_METHOD: u32 = 2;
pub const OPCUA_TIMEOUT_SECS: u64 = 3;

/// NodeIDs por defecto
pub const DEFAULT_HEARTBEAT_NODE: &str = "ns=4;i=3";
pub const DEFAULT_SEGREGATION_NODE: &str = "ns=4;i=2";

/// Nombres de suscripciones
pub const HEARTBEAT_SUBSCRIPTION: &str = "heartbeat_subscription";
pub const SEGREGATION_SUBSCRIPTION: &str = "segregation_subscription";
pub const DEFAULT_SUBSCRIPTION: &str = "default_subscription";
pub const WAGO_VECTOR_SUBSCRIPTION: &str = "wago_vector_subscription";

/// Nodos de prueba WAGO (escalares)
pub const WAGO_BOLEANO_TEST: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.BoleanoTest";
pub const WAGO_BYTE_TEST: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.ByteTest";
pub const WAGO_ENTERO_TEST: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.EnteroTest";
pub const WAGO_REAL_TEST: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.RealTest";
pub const WAGO_STRING_TEST: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.StringTest";
pub const WAGO_WORD_TEST: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.WordTest";

/// Nodos de prueba WAGO (vectores)
pub const WAGO_VECTOR_BOOL: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.VectorBool";
pub const WAGO_VECTOR_INT: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.VectorInt";
pub const WAGO_VECTOR_WORD: &str = "ns=4;s=|var|WAGO TEST.Application.DB_OPC.VectorWord";

/// Tipos de datos WAGO
#[derive(Debug, Clone)]
pub enum WagoNodeType {
    BoleanoTest,
    ByteTest,
    EnteroTest,
    RealTest,
    StringTest,
    WordTest,
    VectorBool,
    VectorInt,
    VectorWord,
}

impl WagoNodeType {
    pub fn from_node_id(node_id: &str) -> Option<Self> {
        if node_id.contains("BoleanoTest") {
            Some(Self::BoleanoTest)
        } else if node_id.contains("ByteTest") {
            Some(Self::ByteTest)
        } else if node_id.contains("EnteroTest") {
            Some(Self::EnteroTest)
        } else if node_id.contains("RealTest") {
            Some(Self::RealTest)
        } else if node_id.contains("StringTest") {
            Some(Self::StringTest)
        } else if node_id.contains("WordTest") {
            Some(Self::WordTest)
        } else if node_id.contains("VectorBool") {
            Some(Self::VectorBool)
        } else if node_id.contains("VectorInt") {
            Some(Self::VectorInt)
        } else if node_id.contains("VectorWord") {
            Some(Self::VectorWord)
        } else {
            None
        }
    }

    pub fn node_id(&self) -> &'static str {
        match self {
            Self::BoleanoTest => WAGO_BOLEANO_TEST,
            Self::ByteTest => WAGO_BYTE_TEST,
            Self::EnteroTest => WAGO_ENTERO_TEST,
            Self::RealTest => WAGO_REAL_TEST,
            Self::StringTest => WAGO_STRING_TEST,
            Self::WordTest => WAGO_WORD_TEST,
            Self::VectorBool => WAGO_VECTOR_BOOL,
            Self::VectorInt => WAGO_VECTOR_INT,
            Self::VectorWord => WAGO_VECTOR_WORD,
        }
    }
}

impl fmt::Display for WagoNodeType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

/// Helper para construir NodeIDs con namespace
pub fn build_node_id(identifier: u32) -> String {
    format!("ns={};i={}", OPCUA_NAMESPACE, identifier)
}

pub fn build_node_id_string(identifier: &str) -> String {
    format!("ns={};s={}", OPCUA_NAMESPACE, identifier)
}
