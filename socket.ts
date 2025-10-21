import WebSocket from 'ws';

// ===========================
// 🔧 CONFIGURACIÓN
// ===========================
const WS_BASE_URL = 'ws://localhost:8081';
const SORTER_ID = 1; // Cambiar según el sorter que quieras monitorear
const RECONNECT_INTERVAL = 3000; // ms
const MAX_RECONNECT_ATTEMPTS = 10;

// ===========================
// 📊 INTERFACES
// ===========================
interface SKUData {
  id: number;
  sku: string;
  percentage: number;
  is_assigned: boolean;
  is_master_case: boolean;
  sealer_id: number | null;
}

interface SKUAssignedMessage {
  type: 'sku_assigned';
  timestamp: string;
  sorter_id: number;
  data: {
    skus: SKUData[];
  };
}

// ===========================
// 🎨 COLORES PARA TERMINAL
// ===========================
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
};

function log(message: string, color: keyof typeof colors = 'reset') {
  const timestamp = new Date().toLocaleTimeString('es-ES');
  console.log(`${colors[color]}[${timestamp}] ${message}${colors.reset}`);
}

// ===========================
// 🔌 CLASE WEBSOCKET CLIENT
// ===========================
class WebSocketMonitor {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectAttempts = 0;
  private shouldReconnect = true;

  constructor(baseUrl: string, sorterId: number) {
    this.url = `${baseUrl}/ws/assignment_${sorterId}`;
  }

  public connect(): void {
    try {
      log(`🔌 Conectando a: ${this.url}`, 'cyan');
      this.ws = new WebSocket(this.url);

      this.ws.on('open', () => {
        log('✅ Conexión establecida exitosamente', 'green');
        this.reconnectAttempts = 0;
        this.printSeparator();
      });

      this.ws.on('message', (data: WebSocket.Data) => {
        this.handleMessage(data.toString());
      });

      this.ws.on('error', (error: Error) => {
        log(`❌ Error de WebSocket: ${error.message}`, 'red');
      });

      this.ws.on('close', (code: number, reason: string) => {
        log(`🔌 Desconectado (código: ${code}, razón: ${reason || 'N/A'})`, 'yellow');
        this.handleReconnect();
      });
    } catch (error) {
      log(`❌ Error al crear WebSocket: ${error}`, 'red');
      this.handleReconnect();
    }
  }

  private handleMessage(data: string): void {
    try {
      const message: SKUAssignedMessage = JSON.parse(data);

      if (message.type === 'sku_assigned') {
        this.printSKUAssignedMessage(message);
      } else {
        log(`📨 Mensaje desconocido: ${data}`, 'yellow');
      }
    } catch (error) {
      log(`⚠️  Error al parsear mensaje: ${data}`, 'yellow');
    }
  }

  private printSKUAssignedMessage(message: SKUAssignedMessage): void {
    this.printSeparator();
    log(`📨 EVENTO: ${message.type}`, 'magenta');
    log(`⏰ Timestamp: ${message.timestamp}`, 'blue');
    log(`🏭 Sorter ID: ${message.sorter_id}`, 'blue');
    log(`📦 Total SKUs: ${message.data.skus.length}`, 'bright');

    // Estadísticas
    const assigned = message.data.skus.filter(s => s.is_assigned);
    const unassigned = message.data.skus.filter(s => !s.is_assigned);
    
    log(`   ├─ ✅ Asignadas: ${assigned.length}`, 'green');
    log(`   └─ ⭕ Disponibles: ${unassigned.length}`, 'yellow');

    // Mostrar SKUs asignadas
    if (assigned.length > 0) {
      console.log('');
      log('✅ SKUs ASIGNADAS:', 'green');
      assigned.forEach((sku, index) => {
        const isLast = index === assigned.length - 1;
        const prefix = isLast ? '   └─' : '   ├─';
        console.log(
          `${colors.green}${prefix} ${sku.sku}${colors.reset} → Salida ${colors.cyan}${sku.sealer_id}${colors.reset} (ID: ${sku.id})`
        );
      });
    }

    // Mostrar SKUs disponibles (solo las primeras 5 para no saturar)
    if (unassigned.length > 0) {
      console.log('');
      log('⭕ SKUs DISPONIBLES:', 'yellow');
      const toShow = unassigned.slice(0, 5);
      toShow.forEach((sku, index) => {
        const isLast = index === toShow.length - 1 && unassigned.length <= 5;
        const prefix = isLast ? '   └─' : '   ├─';
        console.log(
          `${colors.yellow}${prefix} ${sku.sku}${colors.reset} (ID: ${sku.id})`
        );
      });
      
      if (unassigned.length > 5) {
        console.log(`${colors.yellow}   └─ ... y ${unassigned.length - 5} más${colors.reset}`);
      }
    }

    this.printSeparator();
  }

  private printSeparator(): void {
    console.log(colors.cyan + '═'.repeat(80) + colors.reset);
  }

  private handleReconnect(): void {
    if (!this.shouldReconnect) {
      return;
    }

    if (this.reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
      log(
        `❌ Máximo de intentos de reconexión alcanzado (${MAX_RECONNECT_ATTEMPTS})`,
        'red'
      );
      process.exit(1);
    }

    this.reconnectAttempts++;
    log(
      `🔄 Reintentando conexión en ${RECONNECT_INTERVAL}ms (intento ${this.reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})...`,
      'yellow'
    );

    setTimeout(() => {
      this.connect();
    }, RECONNECT_INTERVAL);
  }

  public disconnect(): void {
    log('🛑 Desconectando...', 'yellow');
    this.shouldReconnect = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

// ===========================
// 🚀 MAIN
// ===========================
function main() {
  console.clear();
  log('═'.repeat(80), 'cyan');
  log('🚀 WEBSOCKET MONITOR - API GREENEX', 'bright');
  log('═'.repeat(80), 'cyan');
  log(`📡 URL: ${WS_BASE_URL}/ws/assignment_${SORTER_ID}`, 'blue');
  log(`🏭 Sorter ID: ${SORTER_ID}`, 'blue');
  log(`🔄 Reconexión automática: Sí (máx ${MAX_RECONNECT_ATTEMPTS} intentos)`, 'blue');
  log('💡 Presiona Ctrl+C para salir', 'yellow');
  log('═'.repeat(80), 'cyan');
  console.log('');

  const monitor = new WebSocketMonitor(WS_BASE_URL, SORTER_ID);
  monitor.connect();

  // Manejar Ctrl+C para cerrar limpiamente
  process.on('SIGINT', () => {
    console.log('');
    log('🛑 Señal de interrupción recibida', 'yellow');
    monitor.disconnect();
    setTimeout(() => {
      log('👋 Adiós!', 'green');
      process.exit(0);
    }, 500);
  });
}

// Ejecutar
main();
