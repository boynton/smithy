all:: bin/smithy

bin/smithy::
	mkdir -p bin
	go build -ldflags "-X github.com/boynton/smithy.ToolVersion=`git describe --tag`" -o bin/smithy github.com/boynton/smithy/cmd/smithy

install:: all
	rm -f $(HOME)/bin/smithy
	cp -p bin/smithy $(HOME)/bin/smithy

test::
#	go test github.com/boynton/smithy/test

proper::
	go fmt github.com/boynton/smithy
	go vet github.com/boynton/smithy
	go fmt github.com/boynton/smithy/cmd/smithy
	go vet github.com/boynton/smithy/cmd/smithy

clean::
	rm -rf bin

go.mod:
	go mod init github.com/boynton/smithy && go mod tidy
