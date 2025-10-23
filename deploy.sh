#!/bin/bash
set -e

echo "🚀 Desplegando API-Greenex a producción..."
echo ""

# 1. Compilar binario optimizado
echo "📦 Compilando binario optimizado..."
go build -ldflags="-s -w" -gcflags="-l=4" -o bin/api-greenex cmd/main.go
echo "✅ Binario compilado: $(ls -lh bin/api-greenex | awk '{print $5}')"
echo ""

# 2. Copiar a servidor de producción
echo "📤 Transfiriendo binario a producción..."
scp bin/api-greenex danich@192.168.121.2:~/api-greenex-new
echo "✅ Binario transferido"
echo ""

# 3. Hacer backup del binario actual y reemplazar
echo "🔄 Actualizando binario en producción..."
ssh danich@192.168.121.2 << 'EOF'
    # Backup del binario anterior
    if [ -f ~/api-greenex ]; then
        cp ~/api-greenex ~/api-greenex.backup.$(date +%Y%m%d_%H%M%S)
        echo "✅ Backup creado"
    fi
    
    # Reemplazar binario
    mv ~/api-greenex-new ~/api-greenex
    chmod +x ~/api-greenex
    echo "✅ Binario actualizado"
EOF
echo ""

# 4. Reiniciar servicio
echo "🔄 Reiniciando servicio..."
ssh danich@192.168.121.2 'sudo systemctl restart api-greenex.service'
echo "✅ Servicio reiniciado"
echo ""

# 5. Verificar estado
echo "📊 Verificando estado del servicio..."
ssh danich@192.168.121.2 'sudo systemctl status api-greenex.service --no-pager -l' || true
echo ""

echo "🎉 Despliegue completado!"
echo ""
echo "📡 Para monitorear en tiempo real, ejecuta:"
echo "   ssh danich@192.168.121.2"
echo "   sudo journalctl -u api-greenex.service -f | grep -E \"Caja #|PLC|Error|reconect\" --line-buffered"
