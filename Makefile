build: main.go
	CGO_ENABLED=0 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-w -s" -o main .
run:
	go run ./
deploy: 
	heroku container:push web -a pikabu-freebies-telegram-bot
	heroku container:release web -a pikabu-freebies-telegram-bot
push: build deploy restart
open:
	heroku open -a pikabu-freebies-telegram-bot
suspend:
	heroku ps:scale web=0 --app pikabu-freebies-telegram-bot 
resume:
	heroku ps:scale web=1 --app pikabu-freebies-telegram-bot 
restart: suspend resume