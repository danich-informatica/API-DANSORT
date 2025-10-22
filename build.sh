#!/bin/bash

# Detiene el script si algún comando falla.
set -e

# Directorio de salida para los binarios.
OUTPUT_DIR="bin"

# Crea el directorio de salida si no existe.
mkdir -p "$OUTPUT_DIR"

echo "🔎 Buscando y compilando paquetes main en ./cmd..."

# Itera sobre cada archivo main.go encontrado dentro del directorio cmd.
for main_path in $(find ./cmd -type f -name main.go); do
    # Obtiene la ruta del directorio que contiene el main.go (ej: cmd/http)
    package_dir=$(dirname "$main_path")

    # Genera un nombre para el binario a partir de la ruta del paquete.
    # Reemplaza './cmd/' con nada y las barras '/' con guiones '-'.
    # Ejemplo: ./cmd/test-datamatrix-flow -> test-datamatrix-flow
    # Ejemplo: ./cmd/plc/test -> plc-test
    binary_name=$(echo "$package_dir" | sed -e 's|^\./cmd/||' -e 's|/|-|g')

    # Define la ruta completa del archivo de salida.
    output_path="$OUTPUT_DIR/$binary_name"

    echo "  -> Compilando '$package_dir' en '$output_path'"

    # Compila el paquete para producción.
    # -ldflags="-s -w" reduce el tamaño del binario eliminando símbolos de debug.
    go build -ldflags="-s -w" -o "$output_path" "$package_dir"
done

echo ""
echo "✅ ¡Compilación finalizada con éxito!"
echo "Los binarios se encuentran en el directorio '$OUTPUT_DIR':"
ls -l "$OUTPUT_DIR"
