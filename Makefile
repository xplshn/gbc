OUT = gbc

all: $(OUT)

$(OUT):
	go build -o $(OUT)

clean:
	rm -f $(OUT) ./a.out

test: clean all
	cd tests && ./\.test

examples: clean all
	cd examples && ./\.test

.PHONY: all clean test examples
