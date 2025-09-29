package web

import (
	"fmt"
	"net/http"
	"API-GREENEX/internal/listeners"
	"API-GREENEX/internal/models"
)

// StatusPageHandler sirve una página web con el estado de todos los nodos WAGO
func StatusPageHandler(service *listeners.OPCUAService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes := []string{
			models.WAGO_BoleanoTest,
			models.WAGO_ByteTest,
			models.WAGO_EnteroTest,
			models.WAGO_RealTest,
			models.WAGO_StringTest,
			models.WAGO_WordTest,
			models.WAGO_VectorBool,
			models.WAGO_VectorInt,
			models.WAGO_VectorWord,
		}

		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="es">
<head>
	<meta charset="UTF-8">
	<meta http-equiv='refresh' content='5'>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Estado WAGO</title>
	<style>
		body {
			font-family: 'Segoe UI', Arial, sans-serif;
			background: linear-gradient(120deg, #e0eafc 0%, #cfdef3 100%);
			margin: 0;
			padding: 0;
		}
		.container {
			max-width: 900px;
			margin: 40px auto;
			background: #fff;
			border-radius: 16px;
			box-shadow: 0 4px 24px rgba(0,0,0,0.08);
			padding: 32px 24px;
		}
		h1 {
			text-align: center;
			color: #2a5298;
			margin-bottom: 32px;
		}
		table {
			width: 100%%;
			border-collapse: collapse;
			margin-bottom: 16px;
		}
		th, td {
			padding: 12px 8px;
			text-align: left;
		}
		th {
			background: #2a5298;
			color: #fff;
			font-weight: 600;
			border-bottom: 2px solid #e0eafc;
		}
		tr:nth-child(even) {
			background: #f4f8fb;
		}
		tr:hover {
			background: #e0eafc;
		}
		.error {
			color: #d32f2f;
			font-weight: bold;
		}
		.ok {
			color: #388e3c;
			font-weight: bold;
		}
		.timestamp {
			font-size: 0.95em;
			color: #666;
		}
		.icon {
			width: 28px;
			height: 28px;
			vertical-align: middle;
			margin-right: 8px;
		}
		.bar {
			display: inline-block;
			height: 18px;
			background: #2a5298;
			border-radius: 6px;
			margin-left: 8px;
		}
		@media (max-width: 600px) {
			.container { padding: 8px; }
			th, td { padding: 8px 4px; font-size: 0.95em; }
			.icon { width: 20px; height: 20px; }
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>Estado de Nodos WAGO</h1>
		<table>
			<tr><th>Tipo</th><th>NodeID</th><th>Valor</th><th>Calidad</th><th>Timestamp</th></tr>
`) 
		for _, nodeID := range nodes {
			data, err := service.ReadNode(nodeID)
			var iconSVG string
			var barHTML string
			// Iconos por tipo de nodo
			switch {
			case nodeID == models.WAGO_BoleanoTest || nodeID == models.WAGO_VectorBool:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><circle cx='12' cy='12' r='10' fill='#388e3c'/><text x='12' y='16' text-anchor='middle' font-size='14' fill='#fff'>B</text></svg>`
			case nodeID == models.WAGO_ByteTest:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><rect x='4' y='4' width='16' height='16' rx='4' fill='#1976d2'/><text x='12' y='16' text-anchor='middle' font-size='14' fill='#fff'>By</text></svg>`
			case nodeID == models.WAGO_EnteroTest || nodeID == models.WAGO_VectorInt:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><rect x='2' y='6' width='20' height='12' rx='6' fill='#fbc02d'/><text x='12' y='16' text-anchor='middle' font-size='14' fill='#fff'>I</text></svg>`
			case nodeID == models.WAGO_RealTest:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><ellipse cx='12' cy='12' rx='10' ry='6' fill='#0288d1'/><text x='12' y='16' text-anchor='middle' font-size='14' fill='#fff'>F</text></svg>`
			case nodeID == models.WAGO_StringTest:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><rect x='3' y='7' width='18' height='10' rx='5' fill='#8e24aa'/><text x='12' y='16' text-anchor='middle' font-size='14' fill='#fff'>S</text></svg>`
			case nodeID == models.WAGO_WordTest || nodeID == models.WAGO_VectorWord:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><rect x='2' y='2' width='20' height='20' rx='6' fill='#ff7043'/><text x='12' y='16' text-anchor='middle' font-size='14' fill='#fff'>W</text></svg>`
			default:
				iconSVG = `<svg class='icon' viewBox='0 0 24 24'><circle cx='12' cy='12' r='10' fill='#90a4ae'/></svg>`
			}
			// Gráfico de barra para valores numéricos escalares
			switch v := data.Value.(type) {
			case int:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/100.0)
			case int16:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/100.0)
			case int32:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/100.0)
			case int64:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/100.0)
			case uint8:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val*2.0)
			case uint16:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/2.0)
			case uint32:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/2.0)
			case uint64:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val/2.0)
			case float32:
				val := float64(v)
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val)
			case float64:
				val := v
				barHTML = fmt.Sprintf(`<div class='bar' style='width:%.0fpx'></div>`, val)
			}
			if err != nil {
				fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td colspan='3' class='error'>Error: %v</td></tr>", iconSVG, nodeID, err)
				continue
			}
			calidadClass := "ok"
			if data.Quality != 0 {
				calidadClass = "error"
			}
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%v %s</td><td class='%s'>%v</td><td class='timestamp'>%v</td></tr>", iconSVG, nodeID, data.Value, barHTML, calidadClass, data.Quality, data.Timestamp)
		}
		fmt.Fprintf(w, `</table>
		<div style='text-align:center;color:#888;font-size:0.95em;'>Actualización automática cada 5 segundos</div>
	</div>
</body>
</html>`)
	}
}
