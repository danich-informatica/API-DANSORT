package db

const SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA = `
	SELECT 
		VIE_CodVarieta AS variedad,
		VIE_CodClasse AS calibre,
		VIE_CodConfezione AS embalaje
	FROM dbo.VW_INT_DANICH_ENVIVO
	WHERE VIE_CodVarieta IS NOT NULL 
	  AND VIE_CodClasse IS NOT NULL 
	  AND VIE_CodConfezione IS NOT NULL;
`

const INSERT_LECTURA_DATAMATRIX_SSMS = `
	INSERT INTO PKG_Pallets_Externos 
	(Salida, Correlativo, Numero_Caja, Fecha_Lectura, Terminado)
	VALUES (@p1, @p2, @p3, @p4, 0)
`
const INSERT_SKU_INTERNAL_DB = `
	INSERT INTO SKU (calibre, variedad, embalaje, estado)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (calibre, variedad, embalaje) 
	DO UPDATE SET estado = EXCLUDED.estado
`

const INSERT_SKU_IF_NOT_EXISTS_INTERNAL_DB = `
	INSERT INTO SKU (calibre, variedad, embalaje, estado)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (calibre, variedad, embalaje) DO NOTHING
`

const SELECT_ALL_SKUS_INTERNAL_DB = `
	SELECT calibre, variedad, embalaje, sku, estado
	FROM SKU
	ORDER BY variedad, calibre, embalaje
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
		color,
		correlativo_pallet
	) VALUES (
		CURRENT_TIMESTAMP,
		$1,
		$2,
		$3,
		$4,
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
	SELECT calibre, variedad, embalaje, sku, estado
	FROM sku
	WHERE estado = true
	ORDER BY variedad, calibre, embalaje
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
		s2.estado AS sku_estado
	FROM sorter s
	JOIN salida sal ON s.id = sal.sorter
	JOIN salida_sku ss ON sal.id = ss.salida_id
	JOIN sku s2 ON ss.calibre = s2.calibre AND ss.variedad = s2.variedad AND ss.embalaje = s2.embalaje
	WHERE s.id = $1
	ORDER BY sal.id
`
const INSERT_SALIDA_SKU_INTERNAL_DB = `
	INSERT INTO salida_sku (salida_id, calibre, variedad, embalaje)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (salida_id, calibre, variedad, embalaje) DO NOTHING
`

const DELETE_SALIDA_SKU_INTERNAL_DB = `
	DELETE FROM salida_sku 
	WHERE salida_id = $1 
	  AND calibre = $2 
	  AND variedad = $3 
	  AND embalaje = $4
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
	)
`

const SELECT_HISTORIAL_DESVIOS_INTERNAL_DB = `
	SELECT 
		sc.correlativo_caja AS box_id,
		CONCAT(c.calibre, '-', c.variedad, '-', c.embalaje) AS sku,
		c.calibre,
		sc.salida_enviada AS sealer,
		sc.llena AS is_sealer_full_type,
		sc.fecha_salida AS created_at,
		s.sorter AS sorter_id
	FROM salida_caja sc
	INNER JOIN caja c ON sc.correlativo_caja = c.correlativo
	INNER JOIN salida s ON sc.id_salida = s.id
	WHERE s.sorter = $1
	ORDER BY sc.fecha_salida DESC
	LIMIT 12
`
