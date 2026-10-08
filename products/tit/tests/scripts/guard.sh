#!/bin/sh
# tit guard (#801): RSA, EC and Ed25519 private keys in PEM and DER, OpenSSH and tit keys, and tokens are refused at
# commit and listed by tit guard; their public keys, key-shaped text that is not a key, and paths in .tit-guard-allow
# go through. Keys are made here (openssl, ssh-keygen), never kept in the repository. Usage: guard.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL guard: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'readme\n' > README
t add README
t commit -m first > /dev/null

refused() {
	file=$1
	what=$2
	t add "$file"
	if t commit -m "with $file" > "$d/out.txt" 2>&1; then
		fail "$file was committed"
	fi
	grep -q "$file:[0-9]*: $what" "$d/out.txt" || fail "$file: $(cat "$d/out.txt")"
	t guard > "$d/guard.txt" 2>&1 && fail "tit guard found nothing in $file"
	grep -q "$file:[0-9]*: $what" "$d/guard.txt" || fail "tit guard on $file: $(cat "$d/guard.txt")"
	t rm -f --cached "$file" > /dev/null
	rm -f "$file"
	echo "ok $what refused ($file)"
}
taken() {
	file=$1
	t add "$file"
	t commit -m "with $file" > "$d/out.txt" 2>&1 || fail "$file was refused: $(cat "$d/out.txt")"
	echo "ok $file taken"
}

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out rsa.pem 2> /dev/null
refused rsa.pem "an RSA private key"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out k.pem 2> /dev/null
openssl rsa -in k.pem -out rsa-traditional.pem -traditional 2> /dev/null || openssl rsa -in k.pem -out rsa-traditional.pem 2> /dev/null
refused rsa-traditional.pem "an RSA private key"
openssl pkey -in k.pem -outform DER -out rsa.der 2> /dev/null
refused rsa.der "a DER private key"
openssl pkey -in k.pem -pubout -out rsa-public.pem 2> /dev/null
rm k.pem
taken rsa-public.pem
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -pkeyopt ec_param_enc:named_curve -out ec.pem 2> /dev/null
refused ec.pem "an EC private key"
# LibreSSL writes explicit curve parameters by default, which seal does not take: still a key
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -pkeyopt ec_param_enc:explicit -out ec-explicit.pem 2> /dev/null
refused ec-explicit.pem "an EC private key"
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -pkeyopt ec_param_enc:named_curve -out k.pem 2> /dev/null
openssl ec -in k.pem -out ec-sec1.pem 2> /dev/null
rm k.pem
refused ec-sec1.pem "an EC private key"
# an Ed25519 key in PKCS #8 (DER: a fixed prefix, then the 32-byte seed), made by hand: LibreSSL makes none
seed=$(openssl rand -hex 32)
printf '302e020100300506032b657004220420%s' "$seed" | perl -ne 'print pack("H*", $_)' > ed.der
refused ed.der "a DER private key"
printf '302e020100300506032b657004220420%s' "$seed" | perl -ne 'print pack("H*", $_)' > ed.raw
{ echo "-----BEGIN PRIVATE KEY-----"; openssl base64 -in ed.raw; echo "-----END PRIVATE KEY-----"; } > ed.pem
rm ed.raw
refused ed.pem "an Ed25519 private key"
ssh-keygen -q -t ed25519 -N '' -C test -f ssh_key > /dev/null
refused ssh_key "an OpenSSH private key"
taken ssh_key.pub
t key "$d/r/tit.key" > /dev/null 2>&1
refused tit.key "a tit private key"
# text shaped like a key that does not parse is not one
{ echo "-----BEGIN PRIVATE KEY-----"; echo "bm90IGEga2V5"; echo "-----END PRIVATE KEY-----"; } > example.txt
taken example.txt
printf 'token = "ghp_%s"\n' "$(printf 'a%.0s' $(seq 1 36))" > config.toml
refused config.toml "a GitHub token"
printf 'aws = AKIA%s\n' "ABCDEFGHIJKLMNOP" > aws.txt
refused aws.txt "an AWS access key"
printf 'not a token: myghp_%s\n' "$(printf 'a%.0s' $(seq 1 36))" > words.txt
taken words.txt
mkdir -p testdata
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out testdata/fixture.pem 2> /dev/null
printf 'testdata/\n' > .tit-guard-allow
t add .tit-guard-allow
taken testdata/fixture.pem
