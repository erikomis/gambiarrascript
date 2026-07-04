# Tier 3 — operadores bitwise, literais hex/oct/bin, range `..`,
# interpolacao de string `${}` e o bloco `finalmente`.

# --- bitwise ---
mostra "12 & 10 = ${12 & 10}"
mostra "12 | 2  = ${12 | 2}"
mostra "12 ^ 15 = ${12 ^ 15}"
mostra "1 << 4  = ${1 << 4}"
mostra "256 >> 4 = ${256 >> 4}"
mostra "~0 = ${~0}"

# --- literais em outras bases ---
mostra "0xFF   = ${0xFF}"
mostra "0b1010 = ${0b1010}"
mostra "0o17   = ${0o17}"

# --- range `..` vira lista (inclusive, cresce ou decresce) ---
mostra "1..5 = ${1..5}"
mostra "5..1 = ${5..1}"

bota total = 0
pra_cada n em 1..10
    bota total = total + n
acabou_finalmente
mostra "soma de 1..10 = ${total}"

# --- finalmente sempre roda (com ou sem erro) ---
arruma
    mostra "tentando..."
    quebra("deu ruim de proposito")
quebrou err
    mostra "peguei: ${texto(err)}"
finalmente
    mostra "limpando a bagunca (sempre roda)"
acabou_finalmente
