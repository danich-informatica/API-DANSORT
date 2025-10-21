-- Script PostgreSQL para crear tablas de la base de datos Greenex
-- Elimina y recrea las tablas en orden correcto

SET search_path TO public;

-- Borramos tablas en orden inverso de dependencias
DROP TABLE IF EXISTS orden_vaciado CASCADE;
DROP TABLE IF EXISTS salida_caja CASCADE;
DROP TABLE IF EXISTS salida_sku CASCADE;
DROP TABLE IF EXISTS orden_fabricacion CASCADE;
DROP TABLE IF EXISTS mesa CASCADE;
DROP TABLE IF EXISTS salida CASCADE;
DROP TABLE IF EXISTS sorter CASCADE;
DROP TABLE IF EXISTS caja CASCADE;
DROP TABLE IF EXISTS pallet CASCADE;
DROP TABLE IF EXISTS sku CASCADE;
DROP TABLE IF EXISTS variedad CASCADE;
DROP TABLE IF EXISTS codigoenvase CASCADE;

-- =======================
-- Tabla de Variedades (mapeo código ↔ nombre)
-- Debe crearse ANTES de sku porque sku.variedad es FK
-- =======================
CREATE TABLE variedad (
    codigo_variedad VARCHAR(100) PRIMARY KEY,
    nombre_variedad VARCHAR(255) NOT NULL,
    CONSTRAINT uq_nombre_variedad UNIQUE (nombre_variedad)
);

CREATE INDEX idx_variedad_nombre ON variedad(nombre_variedad);

-- =======================
-- Catálogo SKU
-- =======================
CREATE TABLE sku (
    calibre   VARCHAR(50)  NOT NULL,
    variedad  VARCHAR(100) NOT NULL,
    embalaje  VARCHAR(50)  NOT NULL,
    dark      INTEGER      NOT NULL DEFAULT 0,
    estado    BOOLEAN      NOT NULL DEFAULT TRUE,
    CONSTRAINT pk_sku PRIMARY KEY (calibre, variedad, embalaje, dark),
    CONSTRAINT fk_sku_variedad FOREIGN KEY (variedad)
        REFERENCES variedad (codigo_variedad) ON DELETE CASCADE
);

-- =======================
-- Pallet
-- =======================
CREATE TABLE pallet (
    correlativo     VARCHAR(50) PRIMARY KEY,
    fecha_creacion  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    estado          VARCHAR(50) NOT NULL DEFAULT 'ACTIVO'
);

-- =======================
-- Caja
-- =======================
CREATE TABLE caja (
    correlativo         VARCHAR(50) PRIMARY KEY,
    fecha_embalaje      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    especie             VARCHAR(100) NOT NULL,
    variedad            VARCHAR(100) NOT NULL,
    calibre             VARCHAR(50)  NOT NULL,
    embalaje            VARCHAR(50)  NOT NULL,
    dark                INTEGER      NOT NULL DEFAULT 0,
    color               VARCHAR(50)  NOT NULL,
    correlativo_pallet  VARCHAR(50),
    CONSTRAINT fk_caja_pallet FOREIGN KEY (correlativo_pallet)
        REFERENCES pallet (correlativo) ON DELETE CASCADE,
    CONSTRAINT fk_caja_sku FOREIGN KEY (calibre, variedad, embalaje, dark)
        REFERENCES sku (calibre, variedad, embalaje, dark) ON DELETE RESTRICT
);

CREATE INDEX idx_caja_variedad ON caja (variedad);
CREATE INDEX idx_caja_calibre ON caja (calibre);
CREATE INDEX idx_caja_embalaje ON caja (embalaje);
CREATE INDEX idx_caja_correlativo_pallet ON caja (correlativo_pallet);

-- =======================
-- Sorter
-- =======================
CREATE TABLE sorter (
    id         INT PRIMARY KEY,
    ubicacion  VARCHAR(100) NOT NULL
);

-- =======================
-- Salida
-- =======================
CREATE TABLE salida (
    id             INT PRIMARY KEY,
    sorter         INT NOT NULL,
    salida_sorter  INT NOT NULL,
    estado         BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT fk_salida_sorter FOREIGN KEY (sorter)
        REFERENCES sorter (id) ON DELETE CASCADE
);
CREATE INDEX idx_salida_sorter ON salida (sorter);

CREATE TABLE salida_sku (
    salida_id   INT NOT NULL,
    calibre     VARCHAR(50)  NOT NULL,
    variedad    VARCHAR(100) NOT NULL,
    embalaje    VARCHAR(50)  NOT NULL,
    dark     INTEGER      NOT NULL DEFAULT 0,
    CONSTRAINT pk_salida_sku PRIMARY KEY (salida_id, calibre, variedad, embalaje, dark),
    CONSTRAINT fk_salida_sku_salida FOREIGN KEY (salida_id)
        REFERENCES salida (id) ON DELETE CASCADE,
    CONSTRAINT fk_salida_sku_sku FOREIGN KEY (calibre, variedad, embalaje, dark)
        REFERENCES sku (calibre, variedad, embalaje, dark) ON DELETE SET NULL
);

-- =======================
-- Mesa
-- =======================
CREATE TABLE mesa (
    idmesa  INT PRIMARY KEY,
    salida  INT NOT NULL,
    CONSTRAINT fk_mesa_salida FOREIGN KEY (salida)
        REFERENCES salida (id) ON DELETE CASCADE
);
CREATE INDEX idx_mesa_salida ON mesa (salida);

-- =======================
-- CodigoEnvase
-- =======================
CREATE TABLE codigoenvase (
    codigo      VARCHAR(50) PRIMARY KEY,
    descripcion VARCHAR(255)
);

-- =======================
-- Orden de Fabricación
-- =======================
CREATE TABLE orden_fabricacion (
    id               INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    numeropales      INT NOT NULL CHECK (numeropales > 0),
    cajasporpale     INT NOT NULL CHECK (cajasporpale > 0),
    cajasporcapa     INT NOT NULL CHECK (cajasporcapa > 0),
    codigoenvase     VARCHAR(50) NOT NULL REFERENCES codigoenvase (codigo),
    codigopale       VARCHAR(50) NOT NULL,
    idprogramaflejado INT NOT NULL,
    id_mesa          INT NOT NULL REFERENCES mesa (idmesa) ON DELETE CASCADE,
    fecha_orden      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_orden_fabricacion_mesa ON orden_fabricacion (id_mesa);
CREATE INDEX idx_orden_fabricacion_fecha ON orden_fabricacion (fecha_orden);

-- =======================
-- Salida_Caja
-- =======================
CREATE TABLE salida_caja (
    correlativo_caja VARCHAR(50) NOT NULL,
    id_salida        INT NOT NULL,
    salida_enviada   INT NOT NULL,
    llena           BOOLEAN NOT NULL DEFAULT FALSE,
    fecha_salida     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    id_fabricacion   INT,
    CONSTRAINT pk_salida_caja PRIMARY KEY (correlativo_caja, id_salida),
    CONSTRAINT fk_salida_caja_caja FOREIGN KEY (correlativo_caja)
        REFERENCES caja (correlativo) ON DELETE CASCADE,
    CONSTRAINT fk_salida_caja_salida FOREIGN KEY (id_salida)
        REFERENCES salida (id) ON DELETE CASCADE
    --CONSTRAINT fk_salida_caja_fabricacion FOREIGN KEY (id_fabricacion)
    --    REFERENCES orden_fabricacion (id) ON DELETE CASCADE
);
CREATE INDEX idx_salida_caja_salida ON salida_caja (id_salida);
CREATE INDEX idx_salida_caja_fabricacion ON salida_caja (id_fabricacion);
CREATE INDEX idx_salida_caja_fecha ON salida_caja (fecha_salida);

-- =======================
-- Orden de Vaciado
-- =======================
CREATE TABLE orden_vaciado (
    id                  INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    idmesa              INT NOT NULL,
    modo                INT NOT NULL CHECK (modo >= 0),
    fecha_envio         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    id_fabricacion_activa INT NOT NULL,
    CONSTRAINT fk_orden_vaciado_mesa FOREIGN KEY (idmesa)
        REFERENCES mesa (idmesa) ON DELETE CASCADE,
    CONSTRAINT fk_orden_vaciado_fabricacion FOREIGN KEY (id_fabricacion_activa)
        REFERENCES orden_fabricacion (id) ON DELETE CASCADE
);
CREATE INDEX idx_orden_vaciado_mesa ON orden_vaciado (idmesa);
CREATE INDEX idx_orden_vaciado_fabricacion ON orden_vaciado (id_fabricacion_activa);
CREATE INDEX idx_orden_vaciado_fecha ON orden_vaciado (fecha_envio);


COMMENT ON TABLE sku IS 'Catálogo de productos SKU con calibre, variedad y embalaje';
COMMENT ON TABLE pallet IS 'Registro de pallets con su correlativo y fecha de creación';
COMMENT ON TABLE caja IS 'Registro de cajas con información detallada del producto';
COMMENT ON TABLE sorter IS 'Ubicación física de los sorters en la planta';
COMMENT ON TABLE salida IS 'Salidas disponibles de cada sorter';
COMMENT ON TABLE mesa IS 'Mesas de trabajo asociadas a las salidas';
COMMENT ON TABLE orden_fabricacion IS 'Órdenes de fabricación con detalles de producción';
COMMENT ON TABLE salida_caja IS 'Registro de cajas procesadas por cada salida';
COMMENT ON TABLE codigoenvase IS 'Catálogo de códigos de envases disponibles';
COMMENT ON TABLE orden_vaciado IS 'Órdenes de vaciado de mesas';

-- Crear una secuencia para el correlativo
CREATE SEQUENCE IF NOT EXISTS caja_correlativo_seq
    START WITH 1
    INCREMENT BY 1
    NO MAXVALUE
    NO MINVALUE
    CACHE 1;

-- Función que usa la secuencia
CREATE OR REPLACE FUNCTION generar_correlativo_caja_seq()
RETURNS TRIGGER AS $$
BEGIN
    -- Si el correlativo es NULL, usar el siguiente valor de la secuencia
    IF NEW.correlativo IS NULL THEN
        NEW.correlativo := nextval('caja_correlativo_seq');
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Crear el trigger
CREATE TRIGGER trigger_generar_correlativo_seq
BEFORE INSERT ON caja
FOR EACH ROW
EXECUTE FUNCTION generar_correlativo_caja_seq();

-- Sincronizar la secuencia con los datos existentes (si hay registros previos)
DO $$
DECLARE
    max_correlativo INTEGER;
BEGIN
    SELECT MAX(correlativo::INTEGER) 
    INTO max_correlativo
    FROM caja 
    WHERE correlativo ~ '^[0-9]+$';
    
    -- Solo ajustar si hay datos existentes
    IF max_correlativo IS NOT NULL AND max_correlativo > 0 THEN
        PERFORM setval('caja_correlativo_seq', max_correlativo);
    END IF;
END $$;
