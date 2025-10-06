package db

const SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA = `
	SELECT 
		SEP_Varieta AS variedad,
		SEP_Calibro AS calibre,
		SEP_Confezione AS embalaje
	FROM dbo.SEGREGAZIONE_PROGRAMMA;
`
const INSERT_SKU_INTERNAL_DB = `
	INSERT INTO SKU (calibre, variedad, embalaje, estado)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (calibre, variedad, embalaje) 
	DO UPDATE SET estado = true
`

const INSERT_SKU_IF_NOT_EXISTS_INTERNAL_DB = `
	INSERT INTO SKU (calibre, variedad, embalaje, estado)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (calibre, variedad, embalaje) DO NOTHING
`

const SELECT_ALL_SKUS_INTERNAL_DB = `
	SELECT calibre, variedad, embalaje, estado
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
