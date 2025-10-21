-- ============================================================================
-- Migración: Agregar columna 'dark' a PRIMARY KEY de tabla SKU
-- Fecha: 2025-10-21
-- Descripción: Agrega dark (0 o 1) como parte de la PK
-- ADVERTENCIA: Esto requiere recrear la tabla y actualizar foreign keys
-- ============================================================================

BEGIN;

-- 1. Crear tabla temporal con nueva estructura
CREATE TABLE sku_new (
    calibre   VARCHAR(50)  NOT NULL,
    variedad  VARCHAR(100) NOT NULL,
    embalaje  VARCHAR(50)  NOT NULL,
    dark   INTEGER      NOT NULL DEFAULT 0,
    estado    BOOLEAN      NOT NULL DEFAULT TRUE,
    CONSTRAINT pk_sku_new PRIMARY KEY (calibre, variedad, embalaje, dark)
);

-- 2. Copiar datos existentes (dark = 0 por defecto)
INSERT INTO sku_new (calibre, variedad, embalaje, dark, estado)
SELECT calibre, variedad, embalaje, 0, estado
FROM sku;

-- 3. Eliminar tabla antigua (esto eliminará las foreign keys automáticamente)
DROP TABLE sku CASCADE;

-- 4. Renombrar tabla nueva
ALTER TABLE sku_new RENAME TO sku;

-- 5. Recrear índices si existían
CREATE INDEX IF NOT EXISTS idx_sku_estado ON sku(estado);
CREATE INDEX IF NOT EXISTS idx_sku_variedad ON sku(variedad);

-- 6. Verificar estructura
SELECT * FROM sku LIMIT 5;

COMMIT;

-- ============================================================================
-- NOTA: Después de esta migración, necesitarás recrear las foreign keys
-- de las tablas que referencian a sku:
-- - caja (calibre, variedad, embalaje) -> Ahora debe incluir dark
-- - salida_sku (calibre, variedad, embalaje) -> Ahora debe incluir dark
-- ============================================================================

