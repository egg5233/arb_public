SHELL := /bin/bash
export PATH := /usr/local/go/bin:$(PATH)

.PHONY: build-frontend build run dev clean

build-frontend:
	@# CRITICAL: use `npm ci` only. `npm install` is FORBIDDEN per CLAUDE.md
	@# npm security lockdown (compromised axios package). `npm ci` does a clean
	@# install strictly from package-lock.json and does not mutate the lockfile.
	@export NVM_DIR="$${NVM_DIR:-$$HOME/.nvm}"; \
	if [ -s "$$NVM_DIR/nvm.sh" ]; then \
		cd web && source "$$NVM_DIR/nvm.sh" && nvm use 22 && npm ci --silent && npm run build; \
	elif node -v 2>/dev/null | grep -q '^v2[2-9]\|^v[3-9]'; then \
		cd web && npm ci --silent && npm run build; \
	else \
		echo "錯誤：需要 Node.js >= 22。請先執行 install.sh 或安裝 nvm。"; exit 1; \
	fi

build: build-frontend
	go build -o arb ./cmd/main.go

run:
	./arb

dev:
	go run ./cmd/main.go

clean:
	rm -rf arb web/dist
