# HISTORIAL DE VERSIONES DEL SISTEMA DANSORT

Registro de cambios del **Sistema DANSORT** (Backend API-GREENEX + Frontend).

Este documento proporciona una visión consolidada del progreso del sistema, siguiendo el formato [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/) y el estándar de [Versionado Semántico](https://semver.org/lang/es/).

---

## Índice de Versiones

| Versión | Fecha | Descripción |
|---------|-------|-------------|
| 1.5.0 | 2025-12-11 | Validación de SKU mejorada con parámetros dark y linea |
| 1.4.0 | 2025-12-10 | Extensión del modelo SKU con campo linea |
| 1.3.0 | 2025-12-09 | Refactorización del motor de sorting |
| 1.2.0 | 2025-11-27 | Sistema de reintentos con failover |
| 1.1.0 | 2025-11-25 | Sistema de monitoreo de estados en tiempo real |
| 1.0.0 | 2025-11-21 | Primera versión estable |
| 0.15.0 | 2025-11-17 | Estabilidad de conexión a base de datos |
| 0.14.0 | 2025-11-10 | Consulta de datos de cajas y asignación de SKU |
| 0.13.0 | 2025-11-03 | Campo Flejado y sanitización de consultas |
| 0.12.0 | 2025-10-29 | Refactorización de arquitectura base |
| 0.11.0 | 2025-10-13 | Optimización de gestión WebSocket en frontend |
| 0.10.0 | 2025-10-10 | Mejoras en componentes de reasignación |
| 0.9.0 | 2025-10-09 | Integración de WebSocket nativo |
| 0.8.0 | 2025-10-08 | Módulo de monitoreo de altillo |
| 0.7.0 | 2025-10-07 | Mejoras de interfaz y estadísticas de rechazo |
| 0.6.0 | 2025-10-01 | Sistema de clasificación de rechazos |
| 0.5.0 | 2025-09-30 | Gráficos de estadísticas de rechazo |
| 0.4.0 | 2025-06-24 | Sistema de asignación y generador de etiquetas |
| 0.3.0 | 2025-06-11 | Historial de asignaciones y modales |
| 0.2.0 | 2025-05-20 | Mejoras de diseño y navegación |
| 0.1.0 | 2025-04-14 | Configuración inicial del módulo de asignaciones |
| 0.0.1 | 2025-03-18 | Primer commit del sistema |

---

## [1.5.0] - 2025-12-11

### Agregado
- **Validación de SKU mejorada:** Se extiende la función `CheckSKUExists` para incorporar los parámetros `dark` y `linea`, permitiendo validaciones más precisas en el sistema de identificación de productos.
---

## [1.4.0] - 2025-12-10

### Agregado
- **Extensión del modelo SKU:** Se incorpora el campo `linea` al modelo de SKU junto con la lógica de asignación correspondiente para soportar la segmentación por líneas de producción.

### Mejorado
- **Optimización del listener Cognex:** Se refactoriza el módulo de escucha de cámaras Cognex para mejorar el rendimiento en escenarios de alta carga de lecturas.

---

## [1.3.0] - 2025-12-09

### Mejorado
- **Refactorización del motor de sorting:** Se reestructura la lógica del sorter y se optimiza el protocolo de comunicación con el PLC para reducir latencias.
- **Interfaz de usuario:** Se implementan mejoras en el frontend embebido para una mejor experiencia de monitoreo.

### Agregado
- **Propiedad 'salidas' en SKU:** Se incorpora la nueva propiedad 'salidas' a la interfaz SKU con mejoras en la visualización de componentes relacionados en el sorter.

---

## [1.2.0] - 2025-11-27

> **Versión mayor del sistema.** Integración completa del sistema de reintentos con failover.

### Agregado
- **Sistema de reintentos con failover:** Se implementa un mecanismo robusto de reintentos para señales PLC con soporte para salidas alternativas en caso de fallo.
- **Calibración de SKU:** Se desarrolla funcionalidad completa de calibración de SKU con nuevo modal interactivo y tabla de datos en el sorter.
- **Componente SideMessage:** Se integra el componente SideMessage en la vista de asignaciones con validaciones mejoradas en la asignación de SKUs.

### Mejorado
- **Visualización de listas:** Se mejora la presentación de elementos en las vistas de listado del sorter.

---

## [1.1.0] - 2025-11-25

### Mejorado
- **Optimización de sockets:** Se simplifica la lógica de conexión de sockets en el módulo del sorter.

---

## [1.0.0] - 2025-11-21

> **Primera versión estable del sistema.**

### Agregado
- **Gestión avanzada de salidas:** Se optimiza el manejo de salidas con validación integral de SKU en el módulo de pallet automático.
- **Soporte para salidas de descarte:** Se implementa el tipo de salida "descarte" con validación de embalaje en el procesamiento de datos.
- **Estadísticas de inscripción Cognex:** Se incorpora componente de lista de estadísticas de inscripción de Cognex para el sorter.

---

## [0.15.0] - 2025-11-17

### Corregido
- **Estabilidad de conexión a base de datos:** Se resuelven problemas de conectividad intermitente con la base de datos.

---

## [0.14.0] - 2025-11-10

### Agregado
- **Consulta de datos de cajas:** Se implementa nueva consulta para obtención de datos de cajas con mejoras en la asignación de SKU dentro del sorter.

### Corregido
- **Lógica de asignación:** Se resuelven inconsistencias en la lógica de procesamiento del sorter.

### Mejorado
- **Reestructuración de componentes:** Se reestructuran componentes de monitoreo y reasignación del sorter.

---

## [0.13.0] - 2025-11-03

### Agregado
- **Campo Flejado:** Se incorpora el campo 'Flejado' en las consultas y en el proceso de creación de órdenes de fabricación.
- **Agrupación de SKUs por familia:** Se implementa agrupación de SKUs por familia con mejoras en la visualización de gráficos del sorter.

### Corregido
- **Sanitización de consultas SQL:** Se corrigen espacios en consultas SQL y se mejora el manejo de errores en el envío de órdenes de fabricación.

### Mejorado
- **Estado de existencia:** Se agrega lógica para manejar el estado de existencia en gráficos y tablas de selladoras del sorter.

---

## [0.12.0] - 2025-10-29

> **Primera versión integrada del sistema.**

### Agregado
- **Backend:** Refactorización de arquitectura para mejorar la legibilidad, mantenibilidad y adherencia a patrones de diseño.
- **Backend:** Gráficos de SKUs entrantes con lógica y estilos optimizados.
- **Backend:** Detección automática de baseURL y conexión WebSocket según el segmento de red.
- **Backend:** Filtrado de SKUs por calibrador incluyendo aquellos sin línea asignada.

---

## [0.11.0] - 2025-10-13

### Mejorado
- **Frontend:** Optimización de la gestión del WebSocket eliminando uniones y salidas de sala innecesarias.
- **Frontend:** Mejora en el filtrado de SKUs rechazados y ocultamiento de porcentaje para SKUs de rechazo.
- **Frontend:** Simplificación de la lógica de asignación de SKUs y ocultamiento condicional de botones.

### Agregado
- **Frontend:** Función para obtener SKUs sin rechazo según filtro de selladora automática.
- **Frontend:** Actualización de la configuración del socket de asignaciones con manejo mejorado de rooms.

---

## [0.10.0] - 2025-10-10

### Mejorado
- **Frontend:** Actualización de la configuración de WebSocket en componentes de reasignación.
- **Frontend:** Optimización de estilos y lógica en componentes de reasignación.
- **Frontend:** Mejora en la gestión de conexiones y parámetros de WebSocket.

### Corregido
- **Frontend:** Eliminación de lógica innecesaria en componentes de reasignación.
- **Frontend:** Ajuste de clases y estructura en componentes de formato.

---

## [0.9.0] - 2025-10-09

### Agregado
- **Frontend:** Integración de WebSocket nativo con refactorización completa del módulo.
- **Frontend:** Componente Loftbox con ícono SVG y propiedades de tamaño configurables.

### Mejorado
- **Frontend:** Actualización de lógica y nombres de propiedades en componentes de reasignación.
- **Frontend:** Mejora del diseño de componentes LoftCover y LoftBackground con visualización optimizada de imágenes.

---

## [0.8.0] - 2025-10-08

### Agregado
- **Frontend:** Módulo completo de monitoreo de altillo (Loft) con interfaces, tienda de estado y composables.
- **Frontend:** Componentes LoftCover y LoftBackground para la sección de monitoreo.
- **Frontend:** Botón "Altillo" en el menú principal para acceder a la nueva sección.
- **Frontend:** Acciones de fetch para obtener datos del loft en tiempo real.

---

## [0.7.0] - 2025-10-07

### Agregado
- **Frontend:** Componente RejectsStats para estadísticas de rechazo en el menú principal.

### Mejorado
- **Frontend:** Mejoras de estilo y presentación en múltiples componentes: SystemAccess, PrinterBoxData, PrinterEtiquette, SectionDevices, RejectionsCard, SagCounters, ReassignmentBox, ReassignmentEmptyMessage y ReassignmentFormat.
- **Frontend:** Ajuste de tamaños de íconos y fuentes para mejor presentación visual.
- **Frontend:** Optimización de márgenes y alineaciones en componentes de monitoreo.

---

## [0.6.0] - 2025-10-01

### Agregado
- **Frontend:** Enumeración de razones de rechazo con clasificación automática.
- **Frontend:** Getter para obtener los primeros seis rechazos con filtrado optimizado.

### Mejorado
- **Frontend:** Lógica mejorada para clasificar razones de rechazo.
- **Frontend:** Actualización de versión de Nuxt y mejoras en la página de monitoreo de rechazos.

---

## [0.5.0] - 2025-09-30

### Agregado
- **Frontend:** Componente RejectionsChart para visualización gráfica de estadísticas de rechazo.
- **Frontend:** Página de estadísticas de rechazo con encabezados de tabla.
- **Frontend:** Sección de estadísticas de rechazo integrada en el menú de monitoreo.
- **Frontend:** Colores adicionales en las variables CSS para gráficos.

---

## [0.4.0] - 2025-06-24

### Agregado
- **Frontend:** Tienda de estado para asignaciones (assignment store).
- **Frontend:** Constantes para valores de rotación de elementos en selector.
- **Frontend:** Formulario para generador de elementos de etiqueta.

### Mejorado
- **Frontend:** Actualización de estilos y clases en componentes de asignación.

---

## [0.3.0] - 2025-06-11

### Agregado
- **Frontend:** Historial de asignaciones con interfaz completa.
- **Frontend:** Sistema de modales con estados y acciones para apertura/cierre.
- **Frontend:** Componentes de filtro y menú para asignaciones.
- **Frontend:** Constantes auto-importables para el módulo de asignaciones.
- **Frontend:** Componente SideMessage para notificaciones laterales.
- **Frontend:** Utilidades para gestión de modales (getModalStateBool, openModalWithMessage, openModalByNameAndList).

### Mejorado
- **Frontend:** Reestructuración del layout de asignaciones.
- **Frontend:** Integración de interfaces para modal y notificaciones.

---

## [0.2.0] - 2025-05-20

### Agregado
- **Frontend:** Ícono de apagado (off icon) para el sistema.
- **Frontend:** Componente SideMessage integrado.

### Mejorado
- **Frontend:** Actualización de estilos del layout principal.
- **Frontend:** Mejoras en la barra de navegación (navbar).
- **Frontend:** Nuevos estilos globales.

---

## [0.1.0] - 2025-04-14

### Agregado
- **Frontend:** Configuración inicial del módulo de asignaciones.
- **Frontend:** Tipos e interfaces para asignaciones.
- **Frontend:** Tienda de estado (store) para asignaciones.
- **Frontend:** Componentes base de asignaciones y enumeradores.
- **Frontend:** Layout por defecto y navbar.
- **Frontend:** Íconos y clases personalizadas.
- **Frontend:** Dependencias iniciales del proyecto.

---

## [0.0.1] - 2025-03-18

> **Primer commit del sistema.**

### Agregado
- **Frontend:** Configuración inicial del módulo.
- **Frontend:** Estructura base del proyecto.

---

## Convención de Versionado

El sistema DANSORT utiliza versionado semántico:

| Tipo de Cambio | Descripción |
|----------------|-------------|
| **MAYOR** (X.0.0) | Cambios significativos que afectan la compatibilidad o funcionalidad principal |
| **MENOR** (0.X.0) | Nueva funcionalidad que mantiene compatibilidad hacia atrás |
| **PARCHE** (0.0.X) | Correcciones de errores y mejoras menores |

---

## Arquitectura del Sistema

```
┌─────────────────────────────────────────────────────────────┐
│                    SISTEMA DANSORT                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   FRONTEND                           │   │
│  │                                                      │   │
│  │  • Nuxt.js          • Pinia (Estado)                │   │
│  │  • Vue 3            • WebSocket Client              │   │
│  │  • TypeScript       • Tailwind CSS                  │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│                     WebSocket / REST                        │
│                           │                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  API-GREENEX                         │   │
│  │                                                      │   │
│  │  • Go               • PostgreSQL                     │   │
│  │  • Gin              • SQL Server                     │   │
│  │  • OPC UA           • WebSocket Server               │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│                  ┌────────┴────────┐                       │
│                  │      PLC        │                       │
│                  │    (OPC UA)     │                       │
│                  └─────────────────┘                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Componentes Relacionados

| Componente | Repositorio | Tecnología Principal |
|------------|-------------|---------------------|
| Backend API | API-Greenex | Go / Gin |
| Frontend | demo-dantrack-2025 | Nuxt.js / Vue 3 |
| Documentación | API-Greenex/docs | Markdown |

---

**Versión del Sistema:** `1.5.0`  
**Última Actualización:** 2025-12-11  
**Responsable:** Equipo de Desarrollo DANICH
