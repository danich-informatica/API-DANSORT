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

const SELECT_ALL_SKUS_INTERNAL_DB = `
	SELECT calibre, variedad, embalaje, estado
	FROM SKU
	ORDER BY variedad, calibre, embalaje
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
