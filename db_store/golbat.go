package db_store

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/paulmach/orb/geojson"
	"github.com/sirupsen/logrus"
)

type GolbatDBStore struct {
	logger *logrus.Logger
	db     *sqlx.DB
}

func (st *GolbatDBStore) GetContainedSpawnpoints(ctx context.Context, geom *geojson.Geometry) (spawnpointIds []uint64, err error) {
	const getContainedSpawnpointsQuery = `
SELECT id FROM spawnpoint
    WHERE lat > ? AND lon > ?
		AND lat < ? AND lon < ?
		AND last_seen > UNIX_TIMESTAMP(NOW() - INTERVAL 2 DAY)
		AND ST_CONTAINS(ST_GeomFromGeoJSON(?, 2, 0), POINT(lon, lat))`

	bbox := geom.Geometry().Bound()
	bytes, err := geom.MarshalJSON()
	if err != nil {
		return nil, err
	}

	rows, err := st.db.QueryxContext(ctx, getContainedSpawnpointsQuery, bbox.Min.Lat(), bbox.Min.Lon(), bbox.Max.Lat(), bbox.Max.Lon(), bytes)
	if err != nil {
		return nil, err
	}

	defer func() { err = closeRows(rows, err) }()

	spawnpointIds = make([]uint64, 0, 128)
	var spawnpointId uint64

	for rows.Next() {
		if err = rows.Scan(&spawnpointId); err != nil {
			if err == sql.ErrNoRows {
				err = nil
			}
			spawnpointIds = nil
			return
		}
		spawnpointIds = append(spawnpointIds, spawnpointId)
	}

	return
}

func (st *GolbatDBStore) GetSpawnpointsCount(ctx context.Context, geom *geojson.Geometry) (int64, error) {
	const getContainedSpawnpointsQuery = `
SELECT COUNT(*) FROM spawnpoint
    WHERE lat > ? AND lon > ?
		AND lat < ? AND lon < ?
		AND last_seen > UNIX_TIMESTAMP(NOW() - INTERVAL 7 DAY)
		AND ST_CONTAINS(ST_GeomFromGeoJSON(?, 2, 0), POINT(lon, lat))`

	bbox := geom.Geometry().Bound()
	bytes, err := geom.MarshalJSON()
	if err != nil {
		return 0, err
	}

	row := st.db.QueryRowxContext(ctx, getContainedSpawnpointsQuery, bbox.Min.Lat(), bbox.Min.Lon(), bbox.Max.Lat(), bbox.Max.Lon(), bytes)
	if err != nil {
		return 0, err
	}

	var numSpawnpoints int64

	if err := row.Scan(&numSpawnpoints); err != nil {
		return 0, err
	}

	return numSpawnpoints, nil
}

func NewGolbatDBStore(config DBConfig, logger *logrus.Logger) (*GolbatDBStore, error) {
	db, err := sqlx.Connect("mysql", config.AsDSN())
	if err != nil {
		return nil, err
	}

	if config.MaxPool <= 0 {
		config.MaxPool = 10
	}
	db.SetMaxOpenConns(config.MaxPool)
	db.SetMaxIdleConns(5)

	return &GolbatDBStore{
		logger: logger,
		db:     db,
	}, nil
}
