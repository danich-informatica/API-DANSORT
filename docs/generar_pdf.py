#!/usr/bin/env python3
"""
Script para generar PDF del Historial de Versiones DANSORT

Instrucciones de uso:
1. Instalar dependencias: pip install weasyprint
2. Ejecutar: python3 generar_pdf.py

Alternativas si weasyprint no funciona:
- Opción A: Abrir HISTORIAL_VERSIONES.html en Chrome/Firefox y usar Ctrl+P > "Guardar como PDF"
- Opción B: Instalar wkhtmltopdf: sudo apt install wkhtmltopdf
            Luego ejecutar: wkhtmltopdf --enable-local-file-access HISTORIAL_VERSIONES.html HISTORIAL_VERSIONES.pdf
"""

import os
import sys
from pathlib import Path

def main():
    # Ruta del archivo HTML
    script_dir = Path(__file__).parent
    html_file = script_dir / "HISTORIAL_VERSIONES.html"
    pdf_file = script_dir / "HISTORIAL_VERSIONES_DANSORT.pdf"

    if not html_file.exists():
        print(f"Error: No se encontró el archivo {html_file}")
        sys.exit(1)

    print("=" * 60)
    print("   GENERADOR DE PDF - HISTORIAL DE VERSIONES DANSORT")
    print("=" * 60)

    # Intentar con weasyprint (la mejor opción para HTML a PDF)
    try:
        from weasyprint import HTML
        print("\n✓ Usando WeasyPrint para generar el PDF...")
        HTML(filename=str(html_file)).write_pdf(str(pdf_file))
        print(f"\n✅ PDF generado exitosamente: {pdf_file}")
        return
    except ImportError:
        print("\n⚠️  WeasyPrint no está instalado.")
        print("   Para instalarlo: pip install weasyprint")
    except Exception as e:
        print(f"\n⚠️  Error con WeasyPrint: {e}")

    # Intentar con pdfkit (wrapper de wkhtmltopdf)
    try:
        import pdfkit
        print("\n✓ Usando pdfkit para generar el PDF...")
        options = {
            'page-size': 'Letter',
            'margin-top': '10mm',
            'margin-right': '10mm',
            'margin-bottom': '10mm',
            'margin-left': '10mm',
            'encoding': 'UTF-8',
            'enable-local-file-access': None
        }
        pdfkit.from_file(str(html_file), str(pdf_file), options=options)
        print(f"\n✅ PDF generado exitosamente: {pdf_file}")
        return
    except ImportError:
        print("\n⚠️  pdfkit no está instalado.")
        print("   Para instalarlo: pip install pdfkit")
        print("   También necesitas: sudo apt install wkhtmltopdf")
    except Exception as e:
        print(f"\n⚠️  Error con pdfkit: {e}")

    # Instrucciones manuales
    print("\n" + "=" * 60)
    print("   INSTRUCCIONES PARA GENERAR EL PDF MANUALMENTE")
    print("=" * 60)
    print(f"""
OPCIÓN 1 - Desde el navegador (MÁS FÁCIL):
   1. Abre el archivo en tu navegador:
      {html_file}
   2. Presiona Ctrl+P (o Cmd+P en Mac)
   3. Selecciona "Guardar como PDF" como destino
   4. Guarda como: HISTORIAL_VERSIONES_DANSORT.pdf

OPCIÓN 2 - Instalar WeasyPrint:
   pip install weasyprint
   python3 {__file__}

OPCIÓN 3 - Instalar wkhtmltopdf:
   sudo apt install wkhtmltopdf
   wkhtmltopdf --enable-local-file-access "{html_file}" "{pdf_file}"
""")

if __name__ == "__main__":
    main()

