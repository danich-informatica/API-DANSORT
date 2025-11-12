# API-Greenex

API para la integración con el sistema de paletizado de Danich.

## Descripción

Esta API actúa como un intermediario entre los sistemas de Greenex (UNITEC) y el software de control de paletizado de Danich. Sus responsabilidades principales son:

1.  **Sincronización de SKUs**: Lee periódicamente las SKUs (Stock Keeping Units) activas desde la base de datos de UNITEC y las mantiene actualizadas en una base de datos PostgreSQL local.
2.  **Gestión de Asignaciones**: Proporciona endpoints para que los clientes (como una interfaz de usuario) asignen SKUs a salidas específicas del sorter.
3.  **Comunicación con PLC**: Se comunica con el PLC del sorter para enviar las asignaciones de SKU.
4.  **Recepción de Lecturas**: Recibe lecturas de códigos de caja desde el PLC.
5.  **Proxy a FX_Sync**: Actúa como un proxy para obtener datos de órdenes de fabricación desde la base de datos `FX_Sync` de Serfruit.
6.  **Visualización en Tiempo Real**: Ofrece un WebSocket para monitorizar el estado de las asignaciones, lecturas y estadísticas en tiempo real.

## Arquitectura

El sistema está construido en Go y se compone de varios módulos principales:

-   **`main.go`**: Punto de entrada de la aplicación. Inicializa la configuración, las conexiones a bases de datos, el servidor HTTP y el gestor de WebSockets.
-   **`internal/config`**: Gestiona la carga de configuración desde archivos YAML (`env.yaml`).
-   **`internal/db`**: Contiene toda la lógica de acceso a datos para PostgreSQL (base de datos local) y SQL Server (UNITEC y FX_Sync).
    -   `psql_manager.go`: Conexión a la base de datos local PostgreSQL.
    -   `ssms_manager.go`: Conexión a la base de datos de UNITEC (SQL Server).
    -   `ssms_fx_sync.go`: Conexión a la base de datos `FX_Sync` (SQL Server).
    -   `queries.go`: Define todas las consultas SQL utilizadas en la aplicación.
-   **`internal/flow`**: Lógica de negocio principal.
    -   `sku_manager.go`: Gestiona la sincronización de SKUs desde UNITEC, incluyendo la lógica para manejar inconsistencias de datos y la inserción en la base de datos local.
    -   `assignment_manager.go`: Gestiona la asignación de SKUs a las salidas del sorter.
-   **`internal/web`**: Servidor web y WebSocket.
    -   `server.go`: Define los endpoints de la API REST.
    -   `websocket.go`: Gestiona la comunicación en tiempo real con los clientes.
-   **`internal/listeners`**: Módulos que "escuchan" eventos, como las lecturas del PLC.

### Flujo de Datos de SKU

La construcción de una SKU sigue el formato `CALIBRE-VARIEDAD-EMBALAJE-DARK`.

1.  **Origen de Datos**: La información de las SKUs se obtiene principalmente de la tabla `INT_DANICH_DatosCajas` en la base de datos de UNITEC.
2.  **Sincronización (`syncSKUs`)**:
    -   Un proceso en segundo plano se ejecuta periódicamente (cada 20 segundos por defecto).
    -   Consulta `INT_DANICH_DatosCajas` para obtener las SKUs distintas del proceso más reciente.
    -   Para cada SKU, se extraen `calibre`, `codVariedadTimbrada` (el código de la variedad), `codConfeccion` (embalaje) y `VariedadTimbrada` (el nombre de la variedad).
    -   Se intenta insertar la relación `(codVariedadTimbrada, VariedadTimbrada)` en una tabla local `variedad`. Esto sirve como un mapa para resolver los nombres de variedad.
        -   **Manejo de conflictos**: Si un `nombre_variedad` ya existe, la inserción se omite para evitar errores de "clave duplicada", asegurando que la sincronización no se interrumpa.
    -   Finalmente, la SKU (`calibre`, `variedad` (código), `embalaje`, `dark`) se inserta o actualiza en la tabla local `SKU`.
3.  **Resolución de Nombres**:
    -   Cuando la API necesita mostrar una SKU, utiliza una consulta que une la tabla `SKU` con la tabla `variedad` usando `codigo_variedad`.
    -   La SKU final se construye como `CONCAT(s.calibre, '-', UPPER(COALESCE(v.nombre_variedad, s.variedad)), '-', s.embalaje, '-', s.dark)`.
    -   `COALESCE(v.nombre_variedad, s.variedad)` asegura que si el nombre de la variedad no se pudo resolver (por ejemplo, si no estaba en la tabla `variedad`), se usará el código de la variedad como fallback, evitando que la SKU quede incompleta.

Este mecanismo asegura que, aunque la base de datos de UNITEC pueda tener inconsistencias (como múltiples códigos para un mismo nombre de variedad), la API pueda manejarlo de forma robusta y presentar la información de la manera más clara posible.

## Endpoints de la API

-   `GET /ws`: Inicia una conexión WebSocket.
-   `GET /api/skus`: Devuelve la lista de todas las SKUs activas.
-   `GET /api/assignments/{sorter_id}`: Devuelve las asignaciones de SKU para un sorter determinado.
-   `POST /api/assignments`: Asigna una o más SKUs a una salida.
-   `DELETE /api/assignments`: Elimina una asignación de SKU.
-   `GET /api/outputs`: Devuelve el estado de todas las salidas de todos los sorters.
-   `GET /api/history/deviations/{sorter_id}`: Devuelve un historial de las últimas cajas desviadas.

## Cómo ejecutar

1.  **Configuración**: Asegúrate de que el archivo `config/env.yaml` esté correctamente configurado con las credenciales de las bases de datos.
2.  **Compilar**:
    ```bash
    go build -o bin/api-dansort cmd/main.go
    ```
3.  **Ejecutar**:
    ```bash
    ./bin/api-dansort

