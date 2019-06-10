#!/usr/bin/env bash
curl -X GET http://localhost:8080/log
curl -L -d '{"action":"Create", "data":{"x": "123"}}' -H "Content-Type: application/json" -X POST http://localhost:8080/append