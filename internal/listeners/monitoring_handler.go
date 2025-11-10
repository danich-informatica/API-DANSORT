package listeners

import (
	"net/http"
	"strconv"

	"API-DANSORT/internal/monitoring"

	"github.com/gin-gonic/gin"
)

// MonitoringHandler maneja los endpoints de monitoreo
type MonitoringHandler struct {
	monitor *monitoring.DeviceMonitor
}

// NewMonitoringHandler crea un nuevo handler de monitoreo
func NewMonitoringHandler(monitor *monitoring.DeviceMonitor) *MonitoringHandler {
	return &MonitoringHandler{
		monitor: monitor,
	}
}

// GetSections maneja GET /monitoring/devices/sections
// Retorna el estado de todas las secciones (sorters)
func (h *MonitoringHandler) GetSections(c *gin.Context) {
	sections := h.monitor.GetSectionStatuses()
	c.JSON(http.StatusOK, sections)
}

// GetDevicesBySection maneja GET /monitoring/devices/:section_id
// Retorna todos los dispositivos de una sección específica
func (h *MonitoringHandler) GetDevicesBySection(c *gin.Context) {
	sectionIDStr := c.Param("section_id")
	sectionID, err := strconv.Atoi(sectionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "section_id debe ser un número válido",
		})
		return
	}

	devices := h.monitor.GetDevicesBySection(sectionID)
	c.JSON(http.StatusOK, devices)
}

// GetAllDevices maneja GET /monitoring/devices
// Retorna todos los dispositivos monitoreados (endpoint extra útil)
func (h *MonitoringHandler) GetAllDevices(c *gin.Context) {
	devices := h.monitor.GetAllDevices()
	c.JSON(http.StatusOK, devices)
}
