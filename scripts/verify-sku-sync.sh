#!/bin/bash
# Script de verificación de sincronización de SKUs
# Verifica que las SKUs en PostgreSQL coincidan con lo que muestra el sistema

DB_URL="postgres://danich:danich155@100.111.174.63:5432/greenex?sslmode=disable"

echo "=============================================="
echo "🔍 VERIFICACIÓN DE SINCRONIZACIÓN DE SKUs"
echo "=============================================="
echo ""

echo "📊 1. Conteo de SKUs con estado=true:"
psql "$DB_URL" -c "SELECT COUNT(*) as total_true FROM sku WHERE estado = true;" -t | xargs
echo ""

echo "📊 2. Conteo de SKUs con estado=false:"
psql "$DB_URL" -c "SELECT COUNT(*) as total_false FROM sku WHERE estado = false;" -t | xargs
echo ""

echo "📊 3. Total de SKUs en base de datos:"
psql "$DB_URL" -c "SELECT COUNT(*) as total FROM sku;" -t | xargs
echo ""

echo "📋 4. Listado de SKUs activas (primeras 10):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
psql "$DB_URL" -c "
SELECT
    ROW_NUMBER() OVER (ORDER BY variedad, calibre) as num,
    calibre || '-' || variedad || '-' || embalaje || '-' || dark::text || '-' || linea as sku_key
FROM sku
WHERE estado = true
ORDER BY variedad, calibre
LIMIT 10;
" -t | grep -v '^$'
echo ""

echo "📊 5. Distribución por variedad:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
psql "$DB_URL" -c "
SELECT
    variedad,
    COUNT(*) as total
FROM sku
WHERE estado = true
GROUP BY variedad
ORDER BY variedad;
"
echo ""

echo "📊 6. Distribución por línea:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
psql "$DB_URL" -c "
SELECT
    linea,
    COUNT(*) as total
FROM sku
WHERE estado = true
GROUP BY linea
ORDER BY linea;
"
echo ""

echo "✅ Verificación completada"
echo "=============================================="

