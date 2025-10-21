-- ============================================================================
-- Script de creación de tabla para lecturas DataMatrix
-- Base de datos: FX6_packing_Garate_Operaciones
-- Propósito: Registrar códigos DataMatrix leídos por Cognex en cada salida
-- ============================================================================

USE FX6_packing_Garate_Operaciones;
GO

-- Eliminar tabla si existe (solo para desarrollo/testing)
IF OBJECT_ID('dbo.lectura_datamatrix', 'U') IS NOT NULL
BEGIN
    PRINT '⚠️  Eliminando tabla existente lectura_datamatrix...'
    DROP TABLE dbo.lectura_datamatrix;
END
GO

-- Crear tabla
PRINT '📦 Creando tabla lectura_datamatrix...'
GO

CREATE TABLE dbo.lectura_datamatrix (
    Salida         INT          NOT NULL,  -- ID físico de la salida
    Correlativo    BIGINT       NOT NULL,  -- Código DataMatrix numérico leído
    Numero_Caja    INT          NOT NULL,  -- Contador secuencial por salida
    Fecha_Lectura  DATETIME     NOT NULL DEFAULT GETDATE(),  -- Timestamp de lectura
    Terminado      INT          NOT NULL DEFAULT 0,  -- Estado (0 = pendiente, 1 = procesado)
    
    -- Índices para optimizar consultas
    INDEX IX_Salida (Salida),
    INDEX IX_FechaLectura (Fecha_Lectura DESC),
    INDEX IX_Terminado (Terminado)
);
GO

-- Agregar descripción a la tabla
EXEC sp_addextendedproperty 
    @name = N'MS_Description', 
    @value = N'Registro de lecturas de códigos DataMatrix por salida. Cada lectura incrementa el contador Numero_Caja para esa salida específica.', 
    @level0type = N'SCHEMA', @level0name = N'dbo',
    @level1type = N'TABLE',  @level1name = N'lectura_datamatrix';
GO

-- Agregar descripción a las columnas
EXEC sp_addextendedproperty @name = N'MS_Description', @value = N'ID físico de la salida/sealer', 
    @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'lectura_datamatrix', 
    @level2type = N'COLUMN', @level2name = N'Salida';

EXEC sp_addextendedproperty @name = N'MS_Description', @value = N'Código DataMatrix escaneado por Cognex', 
    @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'lectura_datamatrix', 
    @level2type = N'COLUMN', @level2name = N'Correlativo';

EXEC sp_addextendedproperty @name = N'MS_Description', @value = N'Número secuencial de caja (contador por salida)', 
    @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'lectura_datamatrix', 
    @level2type = N'COLUMN', @level2name = N'Numero_Caja';

EXEC sp_addextendedproperty @name = N'MS_Description', @value = N'Fecha y hora de la lectura', 
    @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'lectura_datamatrix', 
    @level2type = N'COLUMN', @level2name = N'Fecha_Lectura';

EXEC sp_addextendedproperty @name = N'MS_Description', @value = N'Estado: 0 = pendiente, 1 = procesado/terminado', 
    @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'lectura_datamatrix', 
    @level2type = N'COLUMN', @level2name = N'Terminado';
GO

PRINT '✅ Tabla lectura_datamatrix creada exitosamente'
GO

-- Insertar datos de prueba (opcional - comentar si no se necesita)
PRINT '📝 Insertando datos de prueba...'
INSERT INTO dbo.lectura_datamatrix (Salida, Correlativo, Numero_Caja, Fecha_Lectura, Terminado)
VALUES 
    (1, 1001, 1, GETDATE(), 0),
    (1, 1002, 2, DATEADD(SECOND, 5, GETDATE()), 0),
    (7, 2001, 1, DATEADD(SECOND, 10, GETDATE()), 0);
GO

PRINT '✅ Datos de prueba insertados'
GO

-- Consulta de verificación
PRINT '📊 Mostrando datos actuales:'
SELECT TOP 10 
    Salida,
    Correlativo,
    Numero_Caja,
    Fecha_Lectura,
    Terminado
FROM dbo.lectura_datamatrix
ORDER BY Fecha_Lectura DESC;
GO

PRINT '✅ Script completado exitosamente'
GO
