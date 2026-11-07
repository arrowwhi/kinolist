// Package web отдаёт собранный Mini App. Файлы встраиваются в бинарь,
// чтобы прод-образ был одним артефактом без отдельного веб-сервера.
package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var assets embed.FS

// Assets возвращает содержимое dist. Если фронтенд не собран, вернётся
// ошибка — сервис при этом продолжает работать как чистое API.
//
// В git лежит только пустой dist/.gitkeep (его же Vite копирует из public/),
// чтобы embed собирался в свежем клоне. Без index.html отдавать нечего.
func Assets() (fs.FS, error) {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return nil, fmt.Errorf("mini app is not built (run make web-build): %w", err)
	}
	return dist, nil
}
