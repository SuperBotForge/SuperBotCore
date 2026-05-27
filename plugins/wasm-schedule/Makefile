.PHONY: build bundle verify-bundle clean

build:
	GOOS=wasip1 GOARCH=wasm go build -o schedule.wasm ./

bundle:
	@if [ ! -f schedule.wasm ]; then $(MAKE) build; fi
	rm -f schedule-plugin.zip
	tmp=$$(mktemp -d); \
	cp schedule.wasm "$$tmp/plugin.wasm"; \
	mkdir -p "$$tmp/frontend"; \
	cp frontend/index.html frontend/styles.css frontend/app.js "$$tmp/frontend/"; \
	(cd "$$tmp" && zip -Xqr "$(CURDIR)/schedule-plugin.zip" plugin.wasm frontend); \
	rm -rf "$$tmp"

verify-bundle: bundle
	unzip -t schedule-plugin.zip
	unzip -l schedule-plugin.zip

clean:
	rm -f schedule.wasm schedule-plugin.zip
