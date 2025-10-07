# 📋 Guía de Variables de Entorno - API-Greenex

## 🚀 Inicio Rápido

1. Copia el archivo de ejemplo:

```bash
cp .env.example .env
```

2. Edita `.env` con tus valores reales:

```bash
nano .env  # o usa tu editor preferido
```

3. Protege el archivo (recomendado):

```bash
chmod 600 .env
```

## 📚 Documentación Completa de Variables

### 🗄️ PostgreSQL - Base de Datos Principal

Base de datos principal para almacenar SKUs, cajas procesadas y registros del sistema.

| Variable              | Descripción                                   | Valor por Defecto | Requerido |
| --------------------- | --------------------------------------------- | ----------------- | --------- |
| `DANICH_PSQL_DB_URL`  | URL completa de conexión PostgreSQL           | -                 | ❌        |
| `GREENEX_PG_HOST`     | Hostname o IP del servidor PostgreSQL         | `localhost`       | ✅        |
| `GREENEX_PG_PORT`     | Puerto del servidor PostgreSQL                | `5432`            | ✅        |
| `GREENEX_PG_USER`     | Usuario de PostgreSQL                         | `postgres`        | ✅        |
| `GREENEX_PG_PASSWORD` | Contraseña del usuario PostgreSQL             | -                 | ✅        |
| `GREENEX_PG_DATABASE` | Nombre de la base de datos                    | `postgres`        | ✅        |
| `GREENEX_PG_SSLMODE`  | Modo SSL (`disable`, `require`, `verify-ca`)  | `disable`         | ❌        |
| `GREENEX_PG_APP_NAME` | Nombre de la aplicación en logs de PostgreSQL | `api-greenex`     | ❌        |

#### Pool de Conexiones PostgreSQL

| Variable                          | Descripción                                | Valor por Defecto |
| --------------------------------- | ------------------------------------------ | ----------------- |
| `GREENEX_PG_MIN_CONNS`            | Conexiones mínimas en el pool              | `1`               |
| `GREENEX_PG_MAX_CONNS`            | Conexiones máximas en el pool              | `10`              |
| `GREENEX_PG_CONNECT_TIMEOUT`      | Timeout de conexión (formato: `10s`, `1m`) | `10s`             |
| `GREENEX_PG_HEALTHCHECK_INTERVAL` | Intervalo de health check                  | `30s`             |

**Ejemplo de uso:**

```bash
GREENEX_PG_HOST=192.168.1.100
GREENEX_PG_PORT=5432
GREENEX_PG_USER=greenex_user
GREENEX_PG_PASSWORD=SecureP@ssw0rd
GREENEX_PG_DATABASE=greenex_production
```

---

### 🗄️ SQL Server (SSMS) - Base de Datos Calibrador

Base de datos SQL Server para integración con el sistema de calibrador.

| Variable                       | Descripción                                  | Valor por Defecto | Requerido |
| ------------------------------ | -------------------------------------------- | ----------------- | --------- |
| `GREENEX_SSMS_HOST`            | Hostname o IP del SQL Server                 | `localhost`       | ✅        |
| `GREENEX_SSMS_PORT`            | Puerto del SQL Server                        | `1433`            | ✅        |
| `GREENEX_SSMS_DB_USER`         | Usuario de SQL Server                        | `sa`              | ✅        |
| `GREENEX_SSMS_DB_PASSWORD`     | Contraseña del usuario SQL Server            | -                 | ✅        |
| `GREENEX_SSMS_DB_NAME`         | Nombre de la base de datos                   | -                 | ✅        |
| `GREENEX_SSMS_DB_ENCRYPT`      | Encriptación de conexión (`disable`, `true`) | `disable`         | ❌        |
| `GREENEX_SSMS_DB_TRUST_CERT`   | Confiar en certificado del servidor          | `true`            | ❌        |
| `GREENEX_SSMS_APP_NAME`        | Nombre de la aplicación en logs              | `API-Greenex`     | ❌        |
| `GREENEX_SSMS_CONNECT_TIMEOUT` | Timeout de conexión (segundos)               | `15`              | ❌        |

#### Pool de Conexiones SQL Server

| Variable                | Descripción                       | Valor por Defecto |
| ----------------------- | --------------------------------- | ----------------- |
| `DB_MAX_CONNS`          | Conexiones máximas abiertas       | `10`              |
| `DB_MIN_CONNS`          | Conexiones mínimas inactivas      | `5`               |
| `DB_MAX_CONN_LIFETIME`  | Tiempo máximo de vida de conexión | `30m`             |
| `DB_MAX_CONN_IDLE_TIME` | Tiempo máximo de inactividad      | `5m`              |

**Ejemplo de uso:**

```bash
GREENEX_SSMS_HOST=sqlserver.local
GREENEX_SSMS_PORT=1433
GREENEX_SSMS_DB_USER=sa
GREENEX_SSMS_DB_PASSWORD=YourStrongPassword123
GREENEX_SSMS_DB_NAME=CalibradorDB
```

---

### 🔌 OPC UA - Comunicación con WAGO PLC

Configuración para comunicación OPC UA con el PLC WAGO usando protocolo industrial.

#### Conexión Básica

| Variable         | Descripción                                | Valor por Defecto          | Requerido |
| ---------------- | ------------------------------------------ | -------------------------- | --------- |
| `OPCUA_ENDPOINT` | URL del endpoint OPC UA del WAGO           | `opc.tcp://localhost:4840` | ✅        |
| `OPCUA_USERNAME` | Usuario OPC UA (si requiere autenticación) | -                          | ❌        |
| `OPCUA_PASSWORD` | Contraseña OPC UA                          | -                          | ❌        |

**Formato del endpoint:**

```
opc.tcp://[IP_DEL_WAGO]:[PUERTO]
```

Ejemplo: `opc.tcp://192.168.120.16:4840`

#### Seguridad OPC UA

| Variable                 | Descripción               | Valores Posibles                                      | Por Defecto      |
| ------------------------ | ------------------------- | ----------------------------------------------------- | ---------------- |
| `OPCUA_SECURITY_POLICY`  | Política de seguridad     | `None`, `Basic128Rsa15`, `Basic256`, `Basic256Sha256` | `Basic256Sha256` |
| `OPCUA_SECURITY_MODE`    | Modo de seguridad         | `None`, `Sign`, `SignAndEncrypt`                      | `SignAndEncrypt` |
| `OPCUA_CERTIFICATE_PATH` | Ruta al certificado X.509 | -                                                     | -                |
| `OPCUA_PRIVATE_KEY_PATH` | Ruta a la clave privada   | -                                                     | -                |

**Notas de seguridad:**

- Para conexión sin seguridad: `OPCUA_SECURITY_POLICY=None` y `OPCUA_SECURITY_MODE=None`
- Para WAGO con seguridad: Usar `Basic256Sha256` con `SignAndEncrypt`

#### Timeouts y Tiempos

| Variable                   | Descripción         | Formato     | Por Defecto |
| -------------------------- | ------------------- | ----------- | ----------- |
| `OPCUA_CONNECTION_TIMEOUT` | Timeout de conexión | `10s`, `1m` | `10s`       |
| `OPCUA_SESSION_TIMEOUT`    | Timeout de sesión   | `10s`, `1m` | `60s`       |

#### Suscripciones OPC UA

| Variable                      | Descripción                                              | Valor por Defecto |
| ----------------------------- | -------------------------------------------------------- | ----------------- |
| `OPCUA_SUBSCRIPTION_INTERVAL` | Intervalo de publicación de suscripciones                | `500ms`           |
| `OPCUA_KEEPALIVE_COUNT`       | Contador de keep-alive                                   | `3`               |
| `OPCUA_LIFETIME_COUNT`        | Contador de tiempo de vida                               | `300`             |
| `OPCUA_MAX_NOTIFICATIONS`     | Máximo de notificaciones por publicación (0 = ilimitado) | `0`               |

**Ejemplo de configuración completa:**

```bash
OPCUA_ENDPOINT=opc.tcp://192.168.120.16:4840
OPCUA_USERNAME=
OPCUA_PASSWORD=
OPCUA_SECURITY_POLICY=Basic256Sha256
OPCUA_SECURITY_MODE=SignAndEncrypt
OPCUA_CONNECTION_TIMEOUT=10s
OPCUA_SESSION_TIMEOUT=60s
OPCUA_SUBSCRIPTION_INTERVAL=500ms
```

---

### 📷 Cognex Scanner

Configuración del listener TCP para recibir códigos QR/DataMatrix desde el escáner Cognex.

| Variable             | Descripción              | Valor por Defecto   | Requerido |
| -------------------- | ------------------------ | ------------------- | --------- |
| `COGNEX_HOST`        | IP del servidor listener | `0.0.0.0`           | ✅        |
| `COGNEX_PORT`        | Puerto TCP del listener  | `8085`              | ✅        |
| `COGNEX_SCAN_METHOD` | Método de escaneo        | `QR` o `DATAMATRIX` | ✅        |

**Notas:**

- `0.0.0.0` escucha en todas las interfaces de red
- El escáner Cognex debe configurarse para enviar datos a esta IP:Puerto
- `COGNEX_SCAN_METHOD` define el tipo de código que se espera

**Ejemplo:**

```bash
COGNEX_HOST=0.0.0.0
COGNEX_PORT=8085
COGNEX_SCAN_METHOD=QR
```

---

### 🌐 Servidor HTTP - API REST

Configuración del servidor HTTP que expone los endpoints REST.

| Variable    | Descripción              | Valor por Defecto | Requerido |
| ----------- | ------------------------ | ----------------- | --------- |
| `HTTP_PORT` | Puerto del servidor HTTP | `8080`            | ✅        |

**Endpoints disponibles:**

- `GET /Mesa/Estado?id=X` - Consultar estado de mesa
- `POST /Mesa?id=X` - Procesar mesa (fabricación)
- `POST /Mesa/Vaciar?id=X&modo=Y` - Vaciar mesa
- `GET /status` - Página de estado de nodos OPC UA

**Ejemplo:**

```bash
HTTP_PORT=8080
```

Acceso: `http://localhost:8080/status`

---

### 🔀 Sorter - Sistema de Clasificación

Configuración del sistema de clasificación automática (si se usa `cmd/sorter/main.go`).

| Variable           | Descripción                  | Valor por Defecto | Requerido |
| ------------------ | ---------------------------- | ----------------- | --------- |
| `SORTER_ID`        | ID único del sorter          | `1`               | ✅        |
| `SORTER_UBICACION` | Ubicación física del sorter  | `Ubicación 1`     | ✅        |
| `SCAN_METHOD`      | Método de escaneo del sorter | `QR`              | ✅        |
| `SALIDA_1`         | Nombre de la salida 1        | `Salida 1`        | ❌        |
| `SALIDA_2`         | Nombre de la salida 2        | `Salida 2`        | ❌        |
| `SALIDA_3`         | Nombre de la salida 3        | `Salida 3`        | ❌        |

**Ejemplo:**

```bash
SORTER_ID=1
SORTER_UBICACION=Línea de Producción A
SCAN_METHOD=QR
SALIDA_1=Exportación
SALIDA_2=Mercado Nacional
SALIDA_3=Rechazo
```

---

## 🔧 Ejemplos de Configuración por Entorno

### Desarrollo Local

```bash
# PostgreSQL local
GREENEX_PG_HOST=localhost
GREENEX_PG_PORT=5432
GREENEX_PG_USER=postgres
GREENEX_PG_PASSWORD=dev123
GREENEX_PG_DATABASE=greenex_dev

# OPC UA simulador local
OPCUA_ENDPOINT=opc.tcp://localhost:4840
OPCUA_SECURITY_POLICY=None
OPCUA_SECURITY_MODE=None

# Cognex local
COGNEX_HOST=0.0.0.0
COGNEX_PORT=8085

# HTTP
HTTP_PORT=8080
```

### Producción

```bash
# PostgreSQL en servidor dedicado
GREENEX_PG_HOST=192.168.1.100
GREENEX_PG_PORT=5432
GREENEX_PG_USER=greenex_prod
GREENEX_PG_PASSWORD=StrongP@ssw0rd2024
GREENEX_PG_DATABASE=greenex_production
GREENEX_PG_SSLMODE=require
GREENEX_PG_MAX_CONNS=50

# WAGO PLC en red industrial
OPCUA_ENDPOINT=opc.tcp://192.168.120.16:4840
OPCUA_SECURITY_POLICY=Basic256Sha256
OPCUA_SECURITY_MODE=SignAndEncrypt
OPCUA_CERTIFICATE_PATH=/opt/certs/client-cert.pem
OPCUA_PRIVATE_KEY_PATH=/opt/certs/client-key.pem

# Cognex en red de producción
COGNEX_HOST=192.168.120.10
COGNEX_PORT=8085
COGNEX_SCAN_METHOD=QR

# HTTP en puerto estándar
HTTP_PORT=80
```

---

## ⚠️ Notas de Seguridad

1. **Nunca incluyas el archivo `.env` en control de versiones**

   ```bash
   echo ".env" >> .gitignore
   ```

2. **Protege el archivo con permisos restrictivos**

   ```bash
   chmod 600 .env
   ```

3. **Usa contraseñas fuertes** para bases de datos y servicios

4. **En producción:**

   - Habilita SSL/TLS para PostgreSQL
   - Usa certificados válidos para OPC UA
   - Configura firewall para limitar acceso a puertos

5. **Rotar credenciales regularmente**

---

## 🐛 Troubleshooting

### Error: "cliente no está conectado" (OPC UA)

**Causa:** No puede conectar al endpoint OPC UA del WAGO

**Solución:**

1. Verifica que `OPCUA_ENDPOINT` sea correcto
2. Confirma que el WAGO esté encendido y accesible en la red
3. Prueba hacer ping: `ping 192.168.120.16`
4. Verifica firewall: puerto 4840 debe estar abierto
5. Si usa seguridad, verifica `OPCUA_SECURITY_POLICY` y certificados

### Error: "connection refused" (PostgreSQL)

**Causa:** No puede conectar a PostgreSQL

**Solución:**

1. Verifica que PostgreSQL esté corriendo
2. Confirma `GREENEX_PG_HOST` y `GREENEX_PG_PORT`
3. Verifica credenciales: `GREENEX_PG_USER` y `GREENEX_PG_PASSWORD`
4. Confirma que la base de datos existe: `GREENEX_PG_DATABASE`
5. Revisa `pg_hba.conf` en PostgreSQL para permisos de acceso

### Cognex no envía datos

**Causa:** El escáner no está configurado correctamente

**Solución:**

1. Verifica que el Cognex apunte a la IP correcta
2. Confirma que el puerto sea `COGNEX_PORT` (ej: 8085)
3. Revisa la configuración del método de escaneo
4. Verifica logs del sistema con: `journalctl -f`

---

## 📞 Soporte

Para más información o problemas, consulta:

- Documentación interna del proyecto
- Logs del sistema: `journalctl -u api-greenex -f`
- Contacto: equipo de desarrollo

---

**Última actualización:** Octubre 2025
