all: secp256k1 tokenmed

tokenmed:
	go install github.com/tokenme/tokenmed

secp256k1:
	cp -r dependencies/secp256k1/src vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1/src;
	cp -r dependencies/secp256k1/include vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1/include;

install:
	rm -rf /opt/tokenme-ui/*;
	cp -r ui/build/dist/* /opt/tokenme-ui/;
	cp -f /opt/go/bin/tokenmed /usr/local/bin/;
	chmod a+x /usr/local/bin/tokenmed;
