gorkscrew: main.go
	go build -o $@ .

gorkscrew.exe: main.go
	GOOS=windows go build -o $@ .
