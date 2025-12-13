# HISTORIAL DE VERSIONES - SISTEMA DANSORT

**Sistema de clasificación automatizada de cajas para optimización de paletizado**

Este documento detalla el historial completo de versiones del sistema DANSORT, implementado en GREENEX S.A., siguiendo el formato [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y el estándar de [Versionado Semántico](https://semver.org/lang/es/).

## Convención de Versionado

El sistema utiliza un esquema de versionado semántico (X.Y.Z):

- **X (Mayor):** Cambios estructurales mayores o rediseños significativos
- **Y (Menor):** Nuevas funcionalidades o mejoras sustanciales  
- **Z (Parche):** Correcciones menores y ajustes de estabilidad

---

## [1.6.0] - 2025-12-12

### 🔧 Herramienta CLI de sincronización SKU independiente

**Backend:**
- Se implementa herramienta CLI standalone `sku-sync` para sincronización entre SQL Server y PostgreSQL
- Nuevas herramientas de diagnóstico para troubleshooting de sincronización
- Mejoras significativas en el sistema de logging para mejor trazabilidad
- Sistema de verificación de estado de sincronización (`verify-sku-sync.sh`)
- Documentación técnica completa en `DIAGNOSTICO_SKU_SYNC.md`

**Impacto:** Mayor capacidad de diagnóstico y mantenimiento del sistema de sincronización

---

## [1.5.1] - 2025-12-11

### 🐛 Corrección de restricción UNIQUE en salida_sku

**Backend:**
- Se corrige la restricción UNIQUE en tabla `salida_sku` para incluir campo `linea`
- Prevención de conflictos en asignaciones de SKU con misma variedad/calibre/embalaje pero diferentes líneas
- Script de migración: `migration_fix_salida_sku_constraint.sql`

**Impacto:** Resolución de errores críticos de integridad de datos en asignaciones

---

## [1.5.0] - 2025-12-11

### 🎯 Validación mejorada de SKU en sincronización

**Backend:**
- Se implementó un mecanismo de validación más robusto durante el proceso de sincronización de SKU
- Se extiende la función `CheckSKUExists` para incorporar los parámetros `dark` y `linea`
- El sistema verifica la integridad de los datos antes de propagarlos a los sorters
- Reducción significativa de errores de asignación en producción
- Mejora en los logs de sincronización mostrando la clave completa de SKU (incluyendo línea)

**Impacto:** Mayor precisión en la identificación de productos y mejor trazabilidad del sistema

### 📚 Documentación completa del historial de versiones

**Documentación:**
- Se crean múltiples versiones del CHANGELOG para diferentes audiencias:
  - `CHANGELOG.md`: Versión técnica completa
  - `CHANGELOG_SISTEMA.md`: Versión para gerencia
  - `HISTORIAL_VERSIONES_SIMPLE.md`: Versión simplificada
  - `HISTORIAL_VERSIONES_DANSORT.pdf`: Versión ejecutiva en PDF
- Generador automático de PDF con estilos personalizados

**Impacto:** Mejor comunicación del progreso del sistema a todos los stakeholders

---

## [1.4.0] - 2025-12-10

### 📊 Incorporación del campo línea de producción

**Backend:**
- Se agrega el campo `linea` como parte de la clave primaria de SKU
- Incorporación del campo `linea` al modelo de SKU con lógica de asignación correspondiente
- Permite diferenciar productos idénticos provenientes de diferentes líneas de producción
- Actualización de queries SQL y estructura de base de datos

**Frontend:**
- Visualización de línea de producción en interfaces de asignación
- Filtros mejorados por línea

**Impacto:** Mejora en la trazabilidad del proceso y gestión multi-línea

### ⚡ Optimización del listener Cognex

**Backend:**
- Refactorización del módulo de escucha de cámaras Cognex
- Mejora del rendimiento en escenarios de alta carga de lecturas
- Reducción de latencia en procesamiento de DataMatrix

---

## [1.3.0] - 2025-12-09

### 🔧 Optimización del protocolo de comunicación con PLC

**Backend:**
- Reestructuración de la lógica del sorter
- Optimización del protocolo de comunicación con controladores lógicos programables (PLC)
- Reducción de latencia en transmisión de señales de clasificación
- Mayor estabilidad en la comunicación

**Frontend:**
- Actualización de interfaz de usuario para mejor experiencia de monitoreo
- Mejoras en visualización de estado de conexiones PLC

**Impacto:** Sistema más responsivo y confiable en ambiente productivo

---

## [1.2.0] - 2025-11-27

### 🔄 Sistema de reintentos y salidas alternativas

**Backend:**
- Implementación de mecanismo robusto de reintentos para señales PLC
- Sistema de tolerancia a fallos con soporte para salidas alternativas en caso de fallo
- El sistema puede redirigir cajas a salidas alternativas cuando la salida principal presenta problemas
- Mejora en la resiliencia operativa

**Frontend:**
- Visualización de listas optimizada
- Mejora en la presentación de elementos en las vistas de listado

**Impacto:** Mayor continuidad operativa y reducción de paradas por fallos

---

## [1.1.0] - 2025-11-25

### 📺 Mejoras en la interfaz de monitoreo

**Backend:**
- Sistema de monitoreo de estados en tiempo real
- Implementación de agregador de estados de cajas
- Notificaciones push a través de WebSocket para seguimiento en tiempo real

**Frontend:**
- Ajustes en la presentación de información en la interfaz de operador
- Facilitación del seguimiento del estado del sistema en tiempo real
- Mejoras en dashboard de operación

**Impacto:** Mejor visibilidad operativa y respuesta más rápida ante incidencias

---

## [1.0.0] - 2025-11-21 🎉

### ✨ VERSIÓN ESTABLE PARA PRODUCCIÓN

**Primera versión certificada para uso en ambiente productivo.** El sistema alcanza madurez operativa con las siguientes capacidades completas:

#### Características Principales

**Backend:**
- Gestión avanzada de salidas con validación integral de SKU en módulo de pallet automático
- Soporte completo para salidas de descarte con validación de embalaje
- Sistema de clasificación automática de cajas mediante lectura de DataMatrix
- Gestión de rechazos con categorización de motivos
- Asignación dinámica de SKU a salidas de paletizado

**Frontend:**
- Monitoreo en tiempo real del flujo de producción
- Interfaz operativa completa para gestión de salidas
- Sistema de visualización de estado de equipos

**Impacto:** Hito histórico - Sistema completamente operativo en producción

---

## [0.15.0] - 2025-11-17

### 🔧 Estabilización de conexiones a base de datos

**Backend:**
- Corrección de problemas de desconexión intermitente con SQL Server
- Implementación de sistema de reconexión automática
- Garantía de continuidad operativa ante pérdidas de conexión
- Mejora en manejo de pool de conexiones

**Impacto:** Mayor estabilidad y disponibilidad del sistema

---

## [0.14.0] - 2025-11-10

### 📦 Implementación de caché de cajas y generación de SKU

**Backend:**
- Nueva consulta para obtención de datos de cajas
- Sistema de caché para mejorar rendimiento
- Mejoras en la asignación de SKU dentro del sorter
- Generación automática de identificadores SKU

### 🐛 Correcciones
- Resolución de inconsistencias en la lógica de procesamiento
- Corrección de lógica de asignación de cajas a salidas

**Impacto:** Mejor rendimiento y consistencia en asignaciones

---

## [0.13.0] - 2025-11-03

### 📋 Integración de información de flejado

**Backend:**
- Incorporación del campo 'Flejado' en consultas de base de datos
- Integración en proceso de creación de órdenes de fabricación
- Sanitización y corrección de consultas SQL
- Mejora en manejo de errores en envío de órdenes de fabricación

**Impacto:** Mayor trazabilidad en proceso de empaque

---

## [0.12.0] - 2025-10-29

### 🏗️ Refactorización de arquitectura del sistema

**Backend:**
- Reestructuración completa del código base
- Mejora en legibilidad y mantenibilidad del código
- Adherencia a patrones de diseño establecidos
- Separación de responsabilidades en módulos
- Documentación técnica actualizada

**Impacto:** Base de código más mantenible y escalable para futuras mejoras

---

## [0.11.0] - 2025-10-13

### ⚡ Optimización de actualización en tiempo real

**Backend:**
- Ajuste de tiempos de sincronización para prevenir condiciones de carrera
- Incremento de timeout en `AssignLaneToBox`
- Corrección de problema donde el sistema no esperaba respuesta del sorter
- Mejora en manejo de concurrencia

**Impacto:** Mayor confiabilidad en asignaciones en tiempo real

---

## [0.10.0] - 2025-11-03

### Características
- **Integración con sistema de fabricación:** Se implementa la inserción de órdenes de fabricación con vinculación automática del cliente de pallet en salidas automáticas. (`fcffe07`)
- **Soporte multi-cámara DataMatrix:** Se habilita el procesamiento simultáneo de múltiples cámaras DataMatrix con actualización de la lógica del Sorter. (`0fccc16`)

---

## [0.9.0] - 2025-10-30

### Características
- **Sistema de logging avanzado:** Se implementa registro detallado con timestamps de alta precisión en los métodos de comunicación PLC y Cognex. (`0c0aaa9`)
- **Gestión de SKU en salidas automáticas:** Se desarrolla la secuencia completa de ingreso y eliminación de SKU en salidas automáticas. (`4d04b94`)
- **Lógica de mesas de trabajo:** Se implementa nueva arquitectura para el manejo de mesas de trabajo. (`edf11e5`)

---

## [0.8.0] - 2025-10-29

### Características
- **Campo línea en SKU:** Se extiende el modelo SKU con el campo 'linea' y se actualiza la lógica de sincronización y consultas correspondientes. (`3dc4bd0`)

---

## [0.7.0] - 2025-10-27

### Características
- **API de Pallet:** Se implementa la arquitectura inicial del cliente y servidor de API para el sistema de paletizado. (`4820468`)

---

## [0.6.0] - 2025-10-24 al 2025-10-25

### Características
- **Sistema de triggers PLC:** Se incorpora soporte para triggers en la configuración del PLC con mejora en la lógica de espera del método `AssignLaneToBox`. (`6daa5a5`)
- **Reintentos inteligentes:** Se mejora la lógica de reintentos con manejo de errores granular en `AssignLaneToBox`. (`c279701`)
- **Distribución por lotes:** Se implementa algoritmo de distribución por lotes en el enrutamiento de salidas con suite de pruebas. (`e615455`)
- **Calentamiento de métodos PLC:** Se incorpora lógica de warm-up para métodos PLC con ajuste de intervalos de polling. (`15ab262`)
- **Monitoreo de dispositivos:** Se implementa gestión de configuración y funcionalidades de monitoreo de dispositivos. (`44334c0`)

---

## [0.5.0] - 2025-10-22 al 2025-10-23

### Características
- **Worker de Heartbeat PLC:** Se implementa servicio de heartbeat para mantener conexión activa con el PLC. (`605f61e`)
- **Consolidación de avances:** Se integran mejoras incrementales del período de desarrollo. (`54fe435`)

---

## [0.4.0] - 2025-10-21

### Características
- **Campo 'dark' en modelo de datos:** Se extienden las tablas SKU y salida_sku con la columna 'dark' y se actualizan las consultas relacionadas. (`041a17a`)
- **Modo de color en frontend:** Se implementa nuevo manejo de modo de color con lógica mejorada de generación de SKU en interfaces HTML. (`8856d43`)
- **Tabla de variedades:** Se incorpora soporte para la tabla 'variedad' con actualización de la lógica de SKU para incluir nombre de variedad. (`1d2a533`)

### Integración
- **Merge de rama SOLO-SORTER-MANUAL:** Se fusiona pull request #1 incorporando funcionalidad de sorter manual. (`235a9bd`, `86e3ff7`)

---

## [0.3.0] - 2025-10-16 al 2025-10-20

### Características
- **Nuevo frontend:** Se implementa nueva interfaz de usuario. (`210b78a`)
- **Procesamiento de DataMatrix:** Se desarrolla el procesamiento de mensajes DataMatrix con sistema de eventos asociados en Cognex. (`e575853`)
- **Canal de eventos para salidas automáticas:** Se implementa canal de eventos con goroutine dedicada para escuchar eventos en salidas automáticas. (`4373082`)
- **FX6 Manager:** Se desarrolla la inserción y procesamiento de DataMatrix en el gestor FX6. (`96e54cb`)
- **Mejoras en plantillas HTML:** Se actualizan las plantillas con mejoras en la funcionalidad de Salida. (`49d9665`)

---

## [0.2.0] - 2025-10-13 al 2025-10-15

### Características
- **Páginas de error y recursos estáticos:** Se implementan páginas de error personalizadas y favicon. (`13e3781`)
- **Configuración de nodos PLC:** Se incorpora soporte para nodos PLC en configuración de sorters con nuevos campos YAML. (`c8134cc`)
- **Upsert de entidades:** Se mejora la inserción y actualización de salidas/sorters permitiendo actualizaciones en caso de conflicto. (`aa245de`)
- **Nodos OPC UA en salidas:** Se añade soporte para nodos OPC UA en configuración de salidas con documentación. (`2f9fcd9`)
- **Módulo de comunicación OPC UA:** Se implementa el módulo completo de comunicación OPC UA para PLC. (`b87e998`)
- **Conversión de valores OPC UA:** Se desarrolla el sistema de conversión de valores OPC UA. (`cf02a5d`)
- **Descubrimiento de estructura PLC:** Se implementan funcionalidades para asignar salidas a cajas y descubrir estructura del PLC. (`0c57a36`)
- **Control de salidas:** Se desarrollan funciones para bloquear/desbloquear salidas y enviar señales. (`c166d25`)
- **Suscripciones multi-nodo:** Se implementa monitoreo de múltiples nodos para suscripciones PLC. (`372820e`)
- **Gestión de SKU con publicación periódica:** Se implementa gestión de SKU con publicación periódica en sorter. (`e898f2c`)
- **Estadísticas de flujo completas:** Se incluyen todas las SKUs asignadas en estadísticas, incluso las inactivas. (`410c0a4`)
- **ID en estadísticas de flujo:** Se incorpora identificador único a las estadísticas de flujo de SKU. (`a21696b`)
- **Manejador 404 personalizado:** Se implementa manejador personalizado para rutas inexistentes. (`bf69669`)
- **Respuestas estandarizadas:** Se desarrolla manejo estandarizado de respuestas de error y éxito. (`bfd1ad9`)
- **Normalización de tipos de salida:** Se añade campo 'tipo' a la estructura Salida. (`53fb271`)
- **Protección de SKU REJECT:** Se mejora el manejo de SKUs al eliminar, protegiendo SKUs REJECT. (`825e383`, `796fd3b`)
- **Historial de desvíos:** Se implementa endpoint para historial de desvíos con registro de eventos. (`723b957`)
- **Historial de asignaciones:** Se desarrolla endpoint para historial de asignaciones con servidor estático para frontend. (`3c57759`)
- **Campo 'llena' en salida_caja:** Se incorpora campo para marcar salidas llenas. (`788e8f6`, `71f5153`)
- **Validación de rooms WebSocket:** Se actualiza validación para incluir 'history_sorter'. (`6df3a84`)
- **Sincronización SQL Server a PostgreSQL:** Se implementa worker para sincronización periódica de SKUs. (`d006bc7`)
- **Monitor de dispositivos:** Se desarrollan endpoints para estado y conexión de dispositivos. (`15f5168`)

### Correcciones
- **Limpieza de código:** Se eliminan espacios en blanco innecesarios en funciones de procesamiento. (`abd3f0f`)
- **Formato de códigos de error:** Se corrige el formato de códigos de error de negocio. (`c32e579`)

---

## [0.1.0] - 2025-10-06 al 2025-10-09

### Características
- **Listener de Cognex:** Se implementa configuración y lógica para el listener de Cognex y sistema de sorter. (`ae4e7b3`)
- **Eventos de lectura Cognex:** Se desarrolla gestión de eventos de lectura con estadísticas en el sorter. (`179d43a`)
- **Gestión de configuración:** Se implementa gestión de configuración con integración a base de datos. (`578574f`)
- **Streaming de SKU:** Se desarrolla gestión de SKU con streaming eficiente y canales compartidos. (`b064d3a`)
- **Arquitectura del sorter:** Se mejora configuración y arquitectura del sorter. (`b11eacc`)
- **Persistencia de sorter:** Se refactoriza inserción de sorter y salidas en base de datos. (`16fc58e`)
- **Gestión de salida_sku:** Se implementa gestión de SKU en tabla salida_sku. (`20705c6`)
- **Carga de SKUs desde BD:** Se implementa carga de SKUs asignadas desde base de datos. (`3aeff6b`)
- **Frontend HTTP:** Se refactoriza frontend HTTP eliminando servicio OPC UA obsoleto. (`8ee7122`)
- **Monitor WebSocket:** Se implementa monitor WebSocket para API Greenex. (`6afd3b9`)
- **Estadísticas de flujo:** Se desarrolla sistema de estadísticas de flujo y gestión de lotes para SKUs. (`a295c92`)
- **Visualizador de sorter:** Se implementa visualizador de sorter en tiempo real con UI mejorada. (`e08e2bc`)
- **Optimización de eventos:** Se optimiza el registro de eventos en el procesador Cognex. (`6965f9e`)
- **Documentación del sistema:** Se añade documentación completa del sistema en README.md. (`91e4261`)
- **Gestión de salidas Cognex:** Se mejora gestión de salidas con nuevos parámetros de configuración. (`1089a00`)

---

## [0.0.3] - 2025-10-03

### Características
- **Inserción batch de SKUs:** Se implementa gestión de inserción de SKUs en batch con listener para Cognex. (`50f0ffc`)

---

## [0.0.2] - 2025-10-01

### Características
- **Diagrama de modelo de datos:** Se documenta el modelo de datos con diagrama visual. (`c2ec509`)
- **Scripts de base de datos:** Se crean scripts DDL para creación y eliminación de la base de datos Greenex. (`e9d0422`)
- **Conexiones a SQL Server:** Se implementa gestión de conexiones y consultas a SQL Server. (`5b66d57`)
- **Conexiones multi-base de datos:** Se desarrolla gestión de conexiones para PostgreSQL y SQL Server. (`7b67da1`)

---

## [0.0.1] - 2025-09-26 al 2025-09-30

### Características
- **Implementación inicial:** Se desarrolla API-Greenex con servicios OPC UA y HTTP. (`05399cb`)
- **Reestructuración del proyecto:** Se reorganiza el proyecto siguiendo la estructura definida. (`d956663`)
- **Endpoints de Mesa:** Se implementan nuevos endpoints para operaciones de Mesa. (`6594886`)
- **Integración WAGO:** Se desarrolla lectura y escritura de variables WAGO vía HTTP. (`8984daa`)
- **Rutas y respuestas JSON:** Se mejora el manejo de rutas y respuestas JSON para operaciones de Mesa. (`bc22361`)

### Eliminado
- **Archivo main.go obsoleto:** Se elimina archivo main.go y su contenido previo. (`abb436f`)

---

## [0.0.0] - 2025-09-26

### Inicial
- **Inicio del proyecto:** Primer commit estableciendo la base del repositorio. (`bdbafac`)

---

## Convención de Versionado

| Tipo | Formato | Descripción |
|------|---------|-------------|
| **MAYOR** | `X.0.0` | Cambios incompatibles en la API o reestructuración significativa del sistema |
| **MENOR** | `0.X.0` | Nueva funcionalidad compatible con versiones anteriores |
| **PARCHE** | `0.0.X` | Correcciones de errores compatibles con versiones anteriores |

## Categorías de Cambios

- **Características:** Nueva funcionalidad agregada
- **Correcciones:** Errores solucionados
- **Modificado:** Cambios en funcionalidad existente
- **Rendimiento:** Mejoras de rendimiento
- **Eliminado:** Funcionalidad removida
- **Seguridad:** Correcciones de vulnerabilidades
- **Integración:** Fusiones de ramas

---

**Versión Actual:** `1.5.0`  
**Última Actualización:** 2025-12-11
