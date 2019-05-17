#!/usr/bin/env bash

curl -L -d '{"action":"Create", "data":{"x": "123"}}' -H "Content-Type: application/json" -X POST http://localhost:8080/append