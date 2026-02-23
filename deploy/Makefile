ENVS := local dev prod

.PHONY: help
help:
	@echo "Usage: make [env]-[command]"
	@echo "Envs:     $(ENVS)"
	@echo "Commands: up down restart logs ps build clean"
	@echo "Utils:    prod-backup, [env]-ssl-init, [env]-ssl-renew, docker-prune"

# Standard commands pattern: make local-up, dev-logs, etc.
%-up:
	cd $* && docker-compose up -d

%-down:
	cd $* && docker-compose down

%-restart:
	cd $* && docker-compose restart

%-logs:
	cd $* && docker-compose logs -f

%-ps:
	cd $* && docker-compose ps

%-build:
	cd $* && docker-compose build

%-clean:
	cd $* && docker-compose down -v

# SSL Management (dev/prod only)
%-ssl-init:
	@read -p "Enter email: " email; \
	cd $* && docker-compose run --rm certbot certonly --webroot \
		--webroot-path=/var/www/certbot \
		--email $$email \
		--agree-tos \
		--no-eff-email \
		-d $(if $(filter prod,$*),gate149.ru,dev.gate149.ru)
	cd $* && docker-compose restart nginx

%-ssl-renew:
	cd $* && docker-compose run --rm certbot renew
	cd $* && docker-compose restart nginx

# Production Utilities
prod-backup:
	@mkdir -p backups
	cd prod && docker-compose exec -T postgres pg_dump -U postgres prod_app > ../backups/prod_app_$$(date +%Y%m%d_%H%M%S).sql
	cd prod && docker-compose exec -T postgres pg_dump -U postgres prod_kratos > ../backups/prod_kratos_$$(date +%Y%m%d_%H%M%S).sql
	@echo "Backup saved to ./backups/"

build-all:
	for env in $(ENVS); do cd $$env && docker-compose build; done

docker-prune:
	docker system prune -f
