// Package web отдаёт собранный Mini App. Файлы встраиваются в бинарь,
// чтобы прод-образ был одним артефактом без отдельного веб-сервера.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var assets embed.FS

// Assets возвращает содержимое dist. Если фронтенд не собран, вернётся
// ошибка — сервис при этом продолжает работать как чистое API.
func Assets() (fs.FS, error) {
	return fs.Sub(assets, "dist")
}
