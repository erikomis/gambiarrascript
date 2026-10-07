# POO no modelo do Go: treta (struct) + metodo com receiver + combinado
# (interface implicita) + puxadinho (embedding). Sem class, this, new nem
# heranca.

# ---- treta: os campos, um por linha (com valor padrao opcional) ----
treta Ponto
    x
    y = 0
acabou_finalmente

# "construtor" e so convencao: uma gambiarra nova_x devolvendo a treta
gambiarra nova_ponto(x, y)
    funciona Ponto{x: x, y: y}
acabou_finalmente

# ---- metodo: o receiver vem antes do nome; tudo por referencia ----
gambiarra (p Ponto) distancia()
    funciona raiz(p.x * p.x + p.y * p.y)
acabou_finalmente

gambiarra (p Ponto) move(dx, dy = 0)
    p.x += dx
    p.y += dy
acabou_finalmente

bota p = nova_ponto(3, 4)
mostra p                       # Ponto{x: 3, y: 4}
mostra p.distancia()           # 5
p.move(1, 1)                   # muda o proprio p
mostra p                       # Ponto{x: 4, y: 5}
mostra Ponto{1, 2}             # posicional, na ordem dos campos
mostra Ponto{x: 7}             # campo faltando = padrao (y = 0)
mostra Ponto{1, 2} == Ponto{x: 1, y: 2}   # == compara campo a campo

# ---- combinado: quem tem os metodos, satisfaz (sem declarar nada) ----
combinado Forma
    area()
    nome()
acabou_finalmente

treta Quadrado
    lado
acabou_finalmente
gambiarra (q Quadrado) area()
    funciona q.lado * q.lado
acabou_finalmente
gambiarra (q Quadrado) nome()
    funciona "quadrado"
acabou_finalmente

treta Circulo
    raio
acabou_finalmente
gambiarra (c Circulo) area()
    funciona arredonda(pi * c.raio * c.raio)
acabou_finalmente
gambiarra (c Circulo) nome()
    funciona "circulo"
acabou_finalmente

bota formas = [Quadrado{3}, Circulo{2}]
pra_cada f em formas
    mostra "${f.nome()} de area ${f.area()}"
acabou_finalmente
mostra satisfaz(Quadrado{1}, Forma)    # deu_bom
mostra satisfaz(p, Forma)              # deu_ruim: Ponto nao tem area()

# type switch: tipo() de uma instancia e o nome da treta
gambiarra descreve(v)
    escolhe tipo(v)
    caso "Quadrado"
        funciona "quadrado de lado " + texto(v.lado)
    caso "Circulo"
        funciona "circulo de raio " + texto(v.raio)
    se_nao_colar
        funciona "sei la o que e um " + tipo(v)
    acabou_finalmente
acabou_finalmente
mostra descreve(Circulo{5})
mostra descreve(p)

# type assertion: devolve o valor ou quebra explicando o porque
arruma
    como_tipo(p, Forma)
quebrou err
    mostra erro_msg(err)
acabou_finalmente

# ---- puxadinho: uma treta dentro da outra; campos e metodos sobem ----
treta Animal
    nome = "anonimo"
    patas = 4
acabou_finalmente
gambiarra (a Animal) fala()
    funciona a.nome + " faz barulho"
acabou_finalmente
gambiarra (a Animal) apresenta()
    funciona "sou " + a.nome + ", com " + texto(a.patas) + " patas"
acabou_finalmente

treta Cachorro
    Animal                     # puxadinho: puxa nome, patas, fala, apresenta
    raca = "vira-lata"
acabou_finalmente
gambiarra (c Cachorro) fala()  # o mais raso ganha: esse fala() esconde o do Animal
    funciona c.nome + " late"
acabou_finalmente

bota rex = Cachorro{Animal: Animal{nome: "rex"}}
mostra rex                     # Cachorro{Animal: Animal{nome: "rex", patas: 4}, raca: "vira-lata"}
mostra rex.nome                # rex (campo promovido)
mostra rex.fala()              # rex late
mostra rex.Animal.fala()       # rex faz barulho (acesso explicito)
mostra rex.apresenta()         # metodo promovido roda no Animal de dentro
bota rex.patas = 3             # escreve no Animal de dentro
mostra rex.Animal.patas
mostra pra_json(rex)           # o puxadinho e achatado, igual Go
