// Package http provides the REST API adapter (--api).
//
// @title           subtill API
// @version         0.0.1
// @description     Local subscription manager REST API
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package http

//go:generate go tool swag init --v3.1 -g doc.go -o docs --parseDependency --parseInternal
