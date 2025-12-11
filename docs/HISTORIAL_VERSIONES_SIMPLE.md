# Historial de Versiones del Sistema DANSORT

## Resumen Ejecutivo

DANSORT es el sistema de clasificacion automatizada de cajas desarrollado para optimizar el proceso de paletizado. Este documento presenta el historial completo de versiones, detallando las mejoras implementadas y el progreso del desarrollo.

---

## Convencion de Versionado

El sistema utiliza un esquema de versionado semantico compuesto por tres numeros (X.Y.Z):

- **Primer numero (X):** Indica cambios estructurales mayores o redisenos significativos del sistema.
- **Segundo numero (Y):** Representa nuevas funcionalidades o mejoras sustanciales.
- **Tercer numero (Z):** Corresponde a correcciones menores y ajustes de estabilidad.

---

## Registro de Versiones

| Version | Fecha | Descripcion |
|---------|-------|-------------|
| 1.5.0 | 11 de diciembre de 2025 | Validacion mejorada de SKU en sincronizacion |
| 1.4.0 | 10 de diciembre de 2025 | Incorporacion del campo linea de produccion |
| 1.3.0 | 9 de diciembre de 2025 | Optimizacion del protocolo de comunicacion con PLC |
| 1.2.0 | 27 de noviembre de 2025 | Sistema de reintentos y salidas alternativas |
| 1.1.0 | 25 de noviembre de 2025 | Mejoras en la interfaz de monitoreo |
| 1.0.0 | 21 de noviembre de 2025 | Version estable para produccion |
| 0.15.0 | 17 de noviembre de 2025 | Estabilizacion de conexiones a base de datos |
| 0.14.0 | 10 de noviembre de 2025 | Implementacion de cache de cajas y generacion de SKU |
| 0.13.0 | 3 de noviembre de 2025 | Integracion de informacion de flejado |
| 0.12.0 | 29 de octubre de 2025 | Refactorizacion de arquitectura del sistema |
| 0.11.0 | 13 de octubre de 2025 | Optimizacion de actualizacion en tiempo real |
| 0.10.0 | 10 de octubre de 2025 | Mejoras en modulo de reasignacion |
| 0.9.0 | 9 de octubre de 2025 | Implementacion de WebSocket para comunicacion |
| 0.8.0 | 8 de octubre de 2025 | Nuevo modulo de visualizacion de altillo |
| 0.7.0 | 7 de octubre de 2025 | Rediseno de interfaz de usuario |
| 0.6.0 | 1 de octubre de 2025 | Sistema de motivos de rechazo |
| 0.5.0 | 30 de septiembre de 2025 | Graficos estadisticos de rechazos |
| 0.4.0 | 24 de junio de 2025 | Sistema de asignacion y generacion de etiquetas |
| 0.3.0 | 11 de junio de 2025 | Historial de cambios y notificaciones modales |
| 0.2.0 | 20 de mayo de 2025 | Primera iteracion de diseno visual |
| 0.1.0 | 14 de abril de 2025 | Estructura base del proyecto |
| 0.0.1 | 18 de marzo de 2025 | Inicio del desarrollo |

---

## Detalle de Versiones

### Version 1.5.0 - 11 de diciembre de 2025
**Validacion mejorada de SKU en sincronizacion**

Se implemento un mecanismo de validacion mas robusto durante el proceso de sincronizacion de SKU. El sistema ahora verifica la integridad de los datos antes de propagarlos a los sorters, reduciendo errores de asignacion.

---

### Version 1.4.0 - 10 de diciembre de 2025
**Incorporacion del campo linea de produccion**

Se agrego el campo "linea" como parte de la clave primaria de SKU. Esta modificacion permite diferenciar productos identicos provenientes de diferentes lineas de produccion, mejorando la trazabilidad del proceso.

---

### Version 1.3.0 - 9 de diciembre de 2025
**Optimizacion del protocolo de comunicacion con PLC**

Se optimizo la comunicacion entre el sistema y los controladores logicos programables (PLC). Las mejoras incluyen reduccion de latencia y mayor estabilidad en la transmision de senales de clasificacion.

---

### Version 1.2.0 - 27 de noviembre de 2025
**Sistema de reintentos y salidas alternativas**

Se implemento un mecanismo de tolerancia a fallos que permite reintentar asignaciones fallidas. Adicionalmente, el sistema puede redirigir cajas a salidas alternativas cuando la salida principal presenta problemas.

---

### Version 1.1.0 - 25 de noviembre de 2025
**Mejoras en la interfaz de monitoreo**

Se realizaron ajustes en la presentacion de informacion en la interfaz de operador, facilitando el seguimiento del estado del sistema en tiempo real.

---

### Version 1.0.0 - 21 de noviembre de 2025
**Version estable para produccion**

Primera version certificada para uso en ambiente productivo. El sistema alcanza madurez operativa con las siguientes capacidades:

- Clasificacion automatica de cajas mediante lectura de DataMatrix
- Gestion de rechazos con categorizacion de motivos
- Asignacion dinamica de SKU a salidas de paletizado
- Monitoreo en tiempo real del flujo de produccion

---

### Version 0.15.0 - 17 de noviembre de 2025
**Estabilizacion de conexiones a base de datos**

Se corrigieron problemas de desconexion intermitente con SQL Server. Se implemento un sistema de reconexion automatica que garantiza la continuidad operativa.

---

### Version 0.14.0 - 10 de noviembre de 2025
**Implementacion de cache de cajas y generacion de SKU**

Se desarrollo un sistema de cache para optimizar las consultas de informacion de cajas. Adicionalmente, se mejoro el algoritmo de generacion de identificadores SKU.

---

### Version 0.13.0 - 3 de noviembre de 2025
**Integracion de informacion de flejado**

Se incorporo el campo "dark" (flejado) al modelo de datos, permitiendo distinguir productos con y sin este atributo en el proceso de clasificacion.

---

### Version 0.12.0 - 29 de octubre de 2025
**Refactorizacion de arquitectura del sistema**

Se reorganizo la estructura del codigo para mejorar la mantenibilidad y escalabilidad. Esta refactorizacion sienta las bases para futuras expansiones del sistema.

---

### Version 0.11.0 - 13 de octubre de 2025
**Optimizacion de actualizacion en tiempo real**

Se mejoro el rendimiento de las actualizaciones de interfaz, reduciendo el tiempo de respuesta entre eventos del sistema y su visualizacion.

---

### Version 0.10.0 - 10 de octubre de 2025
**Mejoras en modulo de reasignacion**

Se optimizo la funcionalidad de reasignacion de SKU a diferentes salidas, incluyendo mejoras visuales y de usabilidad.

---

### Version 0.9.0 - 9 de octubre de 2025
**Implementacion de WebSocket para comunicacion**

Se implemento el protocolo WebSocket para comunicacion bidireccional en tiempo real entre el servidor y la interfaz de usuario.

---

### Version 0.8.0 - 8 de octubre de 2025
**Nuevo modulo de visualizacion de altillo**

Se agrego una seccion dedicada al monitoreo del proceso en el area de altillo, ampliando la cobertura de supervision del sistema.

---

### Version 0.7.0 - 7 de octubre de 2025
**Rediseno de interfaz de usuario**

Se realizo una actualizacion integral del diseno visual, mejorando la ergonomia y la experiencia de los operadores.

---

### Version 0.6.0 - 1 de octubre de 2025
**Sistema de motivos de rechazo**

Se implemento la categorizacion de rechazos, permitiendo identificar las causas especificas por las que una caja no puede ser clasificada.

---

### Version 0.5.0 - 30 de septiembre de 2025
**Graficos estadisticos de rechazos**

Se incorporaron visualizaciones graficas que permiten analizar tendencias y patrones en los rechazos del sistema.

---

### Version 0.4.0 - 24 de junio de 2025
**Sistema de asignacion y generacion de etiquetas**

Se desarrollo la funcionalidad de asignacion de SKU a salidas de paletizado, junto con la capacidad de generar nuevas etiquetas.

---

### Version 0.3.0 - 11 de junio de 2025
**Historial de cambios y notificaciones modales**

Se implemento el registro de modificaciones realizadas por los operadores y un sistema de notificaciones para eventos importantes.

---

### Version 0.2.0 - 20 de mayo de 2025
**Primera iteracion de diseno visual**

Se establecio la identidad visual del sistema, definiendo la paleta de colores, tipografia y estructura de navegacion.

---

### Version 0.1.0 - 14 de abril de 2025
**Estructura base del proyecto**

Se establecio la arquitectura fundamental del sistema, incluyendo la configuracion del entorno de desarrollo y las dependencias principales.

---

### Version 0.0.1 - 18 de marzo de 2025
**Inicio del desarrollo**

Creacion del repositorio y primeras lineas de codigo del proyecto DANSORT.

---

## Descripcion Funcional del Sistema

DANSORT gestiona el proceso de clasificacion automatizada mediante el siguiente flujo operativo:

1. **Lectura:** Las camaras Cognex capturan el codigo DataMatrix de cada caja.

2. **Identificacion:** El sistema consulta la base de datos para obtener los atributos del producto (calibre, variedad, embalaje, flejado).

3. **Clasificacion:** Se determina la salida de destino segun las reglas de asignacion configuradas.

4. **Ejecucion:** Se envia la senal al PLC para activar el mecanismo de desvio correspondiente.

5. **Registro:** Se almacena la trazabilidad completa de cada operacion.

---

## Informacion del Documento

**Sistema:** DANSORT  
**Version actual:** 1.5.0  
**Fecha de actualizacion:** 11 de diciembre de 2025  
**Desarrollado por:** Equipo de Desarrollo DANICH

---

## Glosario de Terminos

| Termino | Definicion |
|---------|------------|
| **DataMatrix** | Codigo bidimensional impreso en las etiquetas que contiene informacion del producto |
| **PLC** | Controlador Logico Programable; dispositivo que ejecuta las acciones mecanicas de clasificacion |
| **SKU** | Stock Keeping Unit; identificador unico compuesto por calibre, variedad, embalaje y flejado |
| **Sorter** | Modulo de clasificacion que gestiona un conjunto de salidas de paletizado |
| **WebSocket** | Protocolo de comunicacion que permite actualizaciones en tiempo real |

---

*Documento generado para uso interno de Greenex.*
