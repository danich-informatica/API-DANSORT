package pallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client es el cliente HTTP para la API de paletizado automático
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// NewClient crea una nueva instancia del cliente de paletizado
func NewClient(host string, port int, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// GetEstadoMesa obtiene el estado de una mesa específica
// Si id = 0, devuelve el estado de todas las mesas
func (c *Client) GetEstadoMesa(ctx context.Context, idMesa int) ([]EstadoMesa, error) {
	url := fmt.Sprintf("%s%s?id=%d", c.baseURL, EndpointGetEstado, idMesa)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error ejecutando request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta: %w", err)
	}

	// Manejar códigos de estado
	switch resp.StatusCode {
	case http.StatusOK:
		// 200 - Estado de la mesa (puede ser una o múltiples)
		var estados []EstadoMesa
		if err := json.Unmarshal(body, &estados); err != nil {
			// Intentar como objeto único si falla como array
			var estado EstadoMesa
			if err := json.Unmarshal(body, &estado); err != nil {
				return nil, fmt.Errorf("error deserializando respuesta: %w", err)
			}
			return []EstadoMesa{estado}, nil
		}
		return estados, nil

	case 202:
		return nil, ErrMesaNoActiva

	case http.StatusNotFound:
		return nil, ErrMesaNoEncontrada

	case http.StatusInternalServerError:
		return nil, ErrEstadoInterno

	default:
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Endpoint:   EndpointGetEstado,
			Timestamp:  time.Now(),
		}
	}
}

// RegistrarNuevaCaja registra una nueva caja leída en una mesa
func (c *Client) RegistrarNuevaCaja(ctx context.Context, idMesa int, idCaja string) error {
	url := fmt.Sprintf("%s%s?idMesa=%d", c.baseURL, EndpointPostNuevaCaja, idMesa)

	payload := NuevaCajaRequest{
		IDCaja: idCaja,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error ejecutando request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo respuesta: %w", err)
	}

	// Manejar códigos de estado
	switch resp.StatusCode {
	case http.StatusOK:
		// 200 - Caja registrada correctamente
		return nil

	case 202:
		return ErrCajaDuplicada

	case http.StatusBadRequest:
		return ErrFormatoIncorrecto

	case http.StatusNotFound:
		return ErrMesaNoEncontradaCaja

	case http.StatusInternalServerError:
		return ErrRegistroCajaError

	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Endpoint:   EndpointPostNuevaCaja,
			Timestamp:  time.Now(),
		}
	}
}

// CrearOrdenFabricacion inserta una nueva orden de fabricación en una mesa
func (c *Client) CrearOrdenFabricacion(ctx context.Context, idMesa int, orden OrdenFabricacionRequest) error {
	url := fmt.Sprintf("%s%s?id=%d", c.baseURL, EndpointPostOrden, idMesa)

	jsonData, err := json.Marshal(orden)
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error ejecutando request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo respuesta: %w", err)
	}

	// Manejar códigos de estado
	switch resp.StatusCode {
	case http.StatusOK:
		// 200 - Orden creada exitosamente
		return nil

	case 209:
		return ErrMesaNoDisponible

	case http.StatusBadRequest:
		return ErrFormatoOrden

	case http.StatusNotFound:
		return ErrMesaNoEncontradaOrden

	case http.StatusInternalServerError:
		return ErrCrearOrdenError

	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Endpoint:   EndpointPostOrden,
			Timestamp:  time.Now(),
		}
	}
}

// VaciarMesa envía una orden para vaciar una mesa de paletizado
// modo: VaciarModoContinuar (1) o VaciarModoFinalizar (2)
func (c *Client) VaciarMesa(ctx context.Context, idMesa int, modo VaciarMesaMode) error {
	url := fmt.Sprintf("%s%s?id=%d&modo=%d", c.baseURL, EndpointPostVaciar, idMesa, modo)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error ejecutando request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo respuesta: %w", err)
	}

	// Manejar códigos de estado
	switch resp.StatusCode {
	case http.StatusOK:
		// 200 - Solicitud de vaciado registrada correctamente
		return nil

	case 202:
		return ErrMesaYaVacia

	case http.StatusBadRequest:
		return ErrFormatoVaciado

	case http.StatusNotFound:
		return ErrMesaNoEncontradaVaciar

	case http.StatusInternalServerError:
		return ErrVaciarMesaError

	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Endpoint:   EndpointPostVaciar,
			Timestamp:  time.Now(),
		}
	}
}

// Ping verifica la conectividad con la API de paletizado
func (c *Client) Ping(ctx context.Context) error {
	// Intentar obtener el estado de todas las mesas como health check
	_, err := c.GetEstadoMesa(ctx, 0)
	if err != nil {
		// Si el error es "Mesa NO tiene OF activa", la API está respondiendo
		if err == ErrMesaNoActiva {
			return nil
		}
		return err
	}
	return nil
}

// Close cierra las conexiones del cliente
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
