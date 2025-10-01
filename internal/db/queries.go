package db

const SELECT_UNITEC_DB_DBO_SEGREGAZIONE_PROGRAMMA = `
	SELECT 
		SEP_Varieta AS variedad,
		SEP_Calibro AS calibre,
		SEP_Confezione AS embalaje
	FROM dbo.SEGREGAZIONE_PROGRAMMA;
`
