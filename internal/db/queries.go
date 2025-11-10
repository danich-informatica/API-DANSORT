package db

// Query CON VIE_Dark y VIE_Descrizione (intentar primero)
const SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA = `
	SELECT DISTINCT 
		dc.CalibreTimbrado as calibre, 
		dc.codVariedadTimbrada as variedad, 
		dc.codConfeccion as embalaje,
		0 as dark,
		dc.VariedadTimbrada AS nombre_variedad,
		1 AS linea
	FROM DatosCajas dc
	WHERE dc.proceso = (SELECT MAX(proceso) FROM DatosCajas);
`

// Query SIN VIE_Dark (fallback si la columna no existe)
const SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA_FALLBACK = `
	SELECT DISTINCT 
		dc.CalibreTimbrado as calibre, 
		dc.codVariedadTimbrada as variedad, 
		dc.codConfeccion as embalaje,
		0 as dark,
		dc.VariedadTimbrada AS nombre_variedad,
		1 AS linea
	FROM DatosCajas dc
	WHERE dc.proceso = (SELECT MAX(proceso) FROM DatosCajas);
`

const SELECT_BOX_DATA_FROM_UNITEC_DB = `
	SELECT dc.CalibreTimbrado as calibre, dc.VariedadTimbrada as variedad, dc.codConfeccion as embalaje
	FROM DatosCajas dc
	WHERE dc.codCaja = @p1;
`

const INSERT_LECTURA_DATAMATRIX_SSMS = `
	INSERT INTO PKG_Pallets_Externos 
	(Salida, Correlativo, Numero_Caja, Fecha_Lectura, Terminado)
	VALUES (@p1, @p2, @p3, @p4, 1)
`

const INSERT_MESA_INTERNAL_DB = `
	INSERT INTO mesa (idmesa, salida) 
	VALUES ($1, $2) 
	ON CONFLICT (idmesa) DO NOTHING
`

const INSERT_ORDEN_FABRICACION_INTERNAL_DB = `
	INSERT INTO orden_fabricacion (
		id_mesa,
		numeropales,
		cajasporpale,
		cajasporcapa,
		codigoenvase,
		codigopale,
		idprogramaflejado,
		fecha_orden
	) VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
	RETURNING id
`

// Query para obtener datos de orden de fabricación desde vista V_Danish en FX_Sync
const SELECT_V_DANISH_BY_CODIGO_EMBALAJE = `
	SELECT 
		TRIM(CANTIDAD_CAJAS) AS CajasPerPale,
		TRIM([CAJAS POR CAPA]) AS CajasPerCapa,
		TRIM([CODIGO ENVASE]) AS CodigoTipoEnvase,
		ANCHOC,
		LARGOC,
		ALTOC,
		[NOMBRE ENVASE],
		TRIM([CODIGO PALLET]) AS CodigoTipoPale,
		ANCHOP,
		LARGOP,
		ALTOP,
	    FLEJADO AS Flejado
	FROM V_Danish
	WHERE CODIGO_EMBALAJE = @p1
`

const INSERT_SKU_INTERNAL_DB = `
	INSERT INTO SKU (calibre, variedad, embalaje, dark, linea, estado)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (calibre, variedad, embalaje, dark) 
	DO UPDATE SET estado = EXCLUDED.estado, linea = EXCLUDED.linea
`

const INSERT_SKU_IF_NOT_EXISTS_INTERNAL_DB = `
	INSERT INTO SKU (calibre, variedad, embalaje, dark, linea, estado)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (calibre, variedad, embalaje, dark) DO NOTHING
`

const SELECT_ALL_SKUS_INTERNAL_DB = `
	SELECT s.calibre, s.variedad, s.embalaje, s.dark, s.linea,
		CONCAT(s.calibre, '-', UPPER(COALESCE(v.nombre_variedad, s.variedad)), '-', s.embalaje, '-', s.dark) as sku, 
	       s.estado,
	       COALESCE(v.nombre_variedad, s.variedad) as nombre_variedad
	FROM SKU s
	LEFT JOIN variedad v ON s.variedad = v.codigo_variedad
	ORDER BY s.variedad, s.calibre, s.embalaje, s.dark
`
const SELECT_IF_EXISTS_SKU_INTERNAL_DB = `
	SELECT EXISTS(SELECT 1 FROM sku WHERE calibre = $1 AND variedad = $2 AND embalaje = $3)
`

const UPDATE_SKU_STATE_INTERNAL_DB = `
	UPDATE SKU
	SET estado = $4
	WHERE calibre = $1 AND variedad = $2 AND embalaje = $3
	
`
const UPDATE_TO_FALSE_SKU_STATE_INTERNAL_DB = `
	UPDATE SKU
	SET estado = false
	WHERE estado = true
`

const INSERT_CAJA_SIN_CORRELATIVO_INTERNAL_DB = `
	INSERT INTO caja (
		fecha_embalaje,
		especie,
		variedad,
		calibre,
		embalaje,
		dark,
		color,
		correlativo_pallet
	) VALUES (
		CURRENT_TIMESTAMP,
		$1,
		$2,
		$3,
		$4,
		$5,
		'Rojo',
		NULL
	) RETURNING correlativo;
`

const INSERT_SALIDA_CAJA_INTERNAL_DB = `
	INSERT INTO salida_caja (
		correlativo_caja,
		id_salida,
		salida_enviada,
		llena,
		fecha_salida,
		id_fabricacion
	) VALUES (
		$1,
		$2,
		$3,
		$4,
		CURRENT_TIMESTAMP,
		NULL
	)
	ON CONFLICT (correlativo_caja, id_salida) DO UPDATE
	SET 
		salida_enviada = EXCLUDED.salida_enviada,
		llena = EXCLUDED.llena,
		fecha_salida = EXCLUDED.fecha_salida;
`
const SELECT_RECENT_BOXES_INTERNAL_DB = `
	SELECT 
		c.correlativo,
		c.especie,
		s.variedad,
		s.calibre,
		s.embalaje,
		TO_CHAR(c.fecha_hora, 'DD/MM/YYYY HH24:MI:SS') as fecha
	FROM caja c
	INNER JOIN sku s ON c.calibre = s.calibre 
		AND c.variedad = s.variedad 
		AND c.embalaje = s.embalaje
	ORDER BY c.fecha_hora DESC
	LIMIT $1
`
const COUNT_BOXES_INTERNAL_DB = `
	SELECT COUNT(*) FROM caja;
`

const SELECT_ACTIVE_SKUS_INTERNAL_DB = `
	SELECT s.calibre, s.variedad, s.embalaje, s.dark, s.linea,
	       CONCAT(s.calibre, '-', UPPER(COALESCE(v.nombre_variedad, s.variedad)), '-', s.embalaje, '-', s.dark) as sku, 
	       s.estado,
	       COALESCE(v.nombre_variedad, s.variedad) as nombre_variedad
	FROM sku s
	LEFT JOIN variedad v ON s.variedad = v.codigo_variedad
	WHERE s.estado = true
	ORDER BY s.variedad, s.calibre, s.embalaje, s.dark
`

const INSERT_NEW_SORTER_IF_NOT_EXISTS_INTERNAL_DB = `
	INSERT INTO sorter (id, ubicacion)
	VALUES ($1, $2)
	ON CONFLICT (id) 
	DO UPDATE SET ubicacion = EXCLUDED.ubicacion
`

const INSERT_NEW_SALIDA_IF_NOT_EXISTS_INTERNAL_DB = `
	INSERT INTO salida (id, sorter, salida_sorter, estado)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (id) 
	DO UPDATE SET 
		sorter = EXCLUDED.sorter,
		salida_sorter = EXCLUDED.salida_sorter,
		estado = EXCLUDED.estado
`
const SELECT_ALL_SORTERS_AND_OUTPUTS_INTERNAL_DB = `
	SELECT 
		s.id AS sorter_id,
		s.ubicacion AS sorter_ubicacion,
		sal.id AS salida_id,
		sal.salida_sorter AS salida_sorter,
		sal.estado AS salida_estado
	FROM sorter s
	LEFT JOIN salida sal ON s.id = sal.sorter
	ORDER BY s.id, sal.id;
`

const SELECT_ASSIGNED_SKUS_FOR_SORTER_INTERNAL_DB = `
	SELECT 
		sal.id AS salida_id,
		sal.salida_sorter AS salida_sorter,
		sal.estado AS salida_estado,
		ss.calibre AS salida_calibre,
		ss.variedad AS salida_variedad,
		ss.embalaje AS salida_embalaje,
		ss.dark AS salida_dark,
		s2.estado AS sku_estado,
		COALESCE(v.nombre_variedad, ss.variedad) as nombre_variedad
	FROM sorter s
	JOIN salida sal ON s.id = sal.sorter
	JOIN salida_sku ss ON sal.id = ss.salida_id
	JOIN sku s2 ON ss.calibre = s2.calibre AND ss.variedad = s2.variedad AND ss.embalaje = s2.embalaje AND ss.dark = s2.dark
	LEFT JOIN variedad v ON ss.variedad = v.codigo_variedad
	WHERE s.id = $1 AND s2.estado = true
	ORDER BY sal.id
`
const INSERT_SALIDA_SKU_INTERNAL_DB = `
	INSERT INTO salida_sku (salida_id, calibre, variedad, embalaje, dark)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (salida_id, calibre, variedad, embalaje, dark) DO NOTHING
`

const DELETE_SALIDA_SKU_INTERNAL_DB = `
	DELETE FROM salida_sku 
	WHERE salida_id = $1 
	  AND calibre = $2 
	  AND variedad = $3 
	  AND embalaje = $4
	  AND dark = $5
`
const DELETE_ALL_SALIDA_SKUS_INTERNAL_DB = `
    DELETE FROM salida_sku 
    WHERE salida_id = $1
`

const CHECK_SALIDA_SKU_EXISTS_INTERNAL_DB = `
	SELECT EXISTS(
		SELECT 1 FROM salida_sku 
		WHERE salida_id = $1 
		  AND calibre = $2 
		  AND variedad = $3 
		  AND embalaje = $4
		  AND dark = $5
	)
`

const SELECT_HISTORIAL_DESVIOS_INTERNAL_DB = `
	SELECT 
		sc.correlativo_caja AS box_id,
		CONCAT(c.calibre, '-', UPPER(COALESCE(v.nombre_variedad, c.variedad)), '-', c.embalaje, '-', c.dark) AS sku,
		c.calibre,
		sc.salida_enviada AS sealer,
		sc.llena AS is_sealer_full_type,
		sc.fecha_salida AS created_at,
		s.sorter AS sorter_id
	FROM salida_caja sc
	INNER JOIN caja c ON sc.correlativo_caja = c.correlativo
	INNER JOIN salida s ON sc.id_salida = s.id
	LEFT JOIN variedad v ON c.variedad = v.codigo_variedad
	WHERE s.sorter = $1
	ORDER BY sc.fecha_salida DESC
	LIMIT 12
`

// =======================
// Queries para tabla variedad (mapeo código ↔ nombre)
// =======================

const INSERT_VARIEDAD_INTERNAL_DB = `
	INSERT INTO variedad (codigo_variedad, nombre_variedad)
	VALUES ($1, $2)
	ON CONFLICT (codigo_variedad) 
	DO UPDATE SET nombre_variedad = EXCLUDED.nombre_variedad
`

const SELECT_NOMBRE_VARIEDAD_BY_CODIGO = `
	SELECT nombre_variedad FROM variedad WHERE codigo_variedad = $1
`

const SELECT_CODIGO_VARIEDAD_BY_NOMBRE = `
	SELECT codigo_variedad FROM variedad WHERE nombre_variedad = $1
`

const SELECT_ALL_VARIEDADES = `
	SELECT codigo_variedad, nombre_variedad FROM variedad ORDER BY nombre_variedad
`
