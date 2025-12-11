-- Migration: Fix salida_sku UNIQUE constraint after adding 'linea' column
-- Date: 2025-12-11
-- Problem: ON CONFLICT clause fails because the UNIQUE constraint doesn't include 'linea'

-- Step 1: Drop existing constraints (if they exist)
ALTER TABLE salida_sku DROP CONSTRAINT IF EXISTS salida_sku_pkey;
ALTER TABLE salida_sku DROP CONSTRAINT IF EXISTS uq_salida_sku;
ALTER TABLE salida_sku DROP CONSTRAINT IF EXISTS uq_salida_sku_all;
ALTER TABLE salida_sku DROP CONSTRAINT IF EXISTS salida_sku_salida_id_calibre_variedad_embalaje_dark_key;

-- Step 2: Create the correct UNIQUE constraint including 'linea'
ALTER TABLE salida_sku ADD CONSTRAINT uq_salida_sku_all
    UNIQUE (salida_id, calibre, variedad, embalaje, dark, linea);

-- Verify the constraint was created
SELECT conname, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'salida_sku'::regclass;
