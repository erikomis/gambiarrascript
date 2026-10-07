// Realce de sintaxe do GambiarraScript pro CodeMirror 6, no estilo "legacy
// mode" (StreamLanguage): um tokenizador linha a linha, sem gramatica. Segue o
// lexer de verdade (lexer/lexer.go) e as keywords de token/token.go.

import { StreamLanguage, type StringStream } from "@codemirror/language";

// token/token.go (keywords) + `crava` (constante)
const keywords = new Set([
  "bota",
  "crava",
  "mostra",
  "se_colar",
  "se_nao_colar",
  "enquanto",
  "pra_cada",
  "de",
  "ate",
  "em",
  "gambiarra",
  "funciona",
  "arruma",
  "quebrou",
  "vaza",
  "continua",
  "acabou_finalmente",
  "finalmente",
  "escolhe",
  "caso",
  "e",
  "ou",
  "nao",
  "importa",
  "bora",
  "entao",
  "como",
  "treta",
  "combinado",
]);
const booleanos = new Set(["deu_bom", "deu_ruim"]);

type Modo = "codigo" | "texto" | "cru" | "bloco";

interface Estado {
  modo: Modo;
  // uma entrada por `${` aberto dentro de texto: profundidade de { } la dentro
  interp: number[];
  // proximo identificador e nome de funcao sendo declarada (depois de `gambiarra`)
  defineFuncao: boolean;
}

const ident = /^[\p{L}_][\p{L}\p{N}_]*/u;
const numero = /^(?:0[xX][0-9a-fA-F]+|0[oO][0-7]+|0[bB][01]+|\d+(?:\.\d+)?)/;
const operador =
  /^(?:\?\?|\?\.|\.\.\.|\.\.|<<=|>>=|<<|>>|[=!<>+\-*/%&|^]=?|~|\.)/;

// dentro de "...": escapes, interpolacao ${...} e o resto do texto
function tokenTexto(stream: StringStream, estado: Estado): string {
  if (stream.match(/^\\["\\nt]/)) return "string.special";
  if (stream.match("${")) {
    estado.interp.push(0);
    estado.modo = "codigo";
    return "string.special";
  }
  if (stream.eat('"')) {
    estado.modo = "codigo";
    return "string";
  }
  // consome ate o proximo caractere que interessa (ou o fim da linha)
  if (!stream.match(/^[^"\\$]+/)) stream.next();
  return "string";
}

function tokenCodigo(stream: StringStream, estado: Estado): string | null {
  if (stream.eatSpace()) return null;

  // so o token logo depois de `gambiarra` conta (gambiarra(x) anonima nao)
  const nomeDeFuncao = estado.defineFuncao;
  estado.defineFuncao = false;

  if (stream.eat("#")) {
    stream.skipToEnd();
    return "lineComment";
  }
  if (stream.match("/*")) {
    estado.modo = "bloco";
    return tokenBloco(stream, estado);
  }
  if (stream.eat('"')) {
    estado.modo = "texto";
    return "string";
  }
  if (stream.eat("`")) {
    estado.modo = "cru";
    return tokenCru(stream, estado);
  }

  // chaves contam pra saber quando a interpolacao ${...} fecha
  if (estado.interp.length > 0) {
    const topo = estado.interp.length - 1;
    if (stream.eat("{")) {
      estado.interp[topo]++;
      return "brace";
    }
    if (stream.peek() === "}") {
      stream.next();
      if (estado.interp[topo] === 0) {
        estado.interp.pop();
        estado.modo = "texto";
        return "string.special";
      }
      estado.interp[topo]--;
      return "brace";
    }
  }

  if (stream.match(numero)) return "number";

  const m = stream.match(ident) as RegExpMatchArray | null;
  if (m) {
    const palavra = m[0];
    if (nomeDeFuncao) return "variableName.function.definition";
    if (booleanos.has(palavra)) return "bool";
    if (palavra === "nada") return "atom"; // o tema one-dark nao pinta "null"
    if (keywords.has(palavra)) {
      if (palavra === "gambiarra") estado.defineFuncao = true;
      return "keyword";
    }
    // chamada: nome( — pinta como funcao
    if (stream.match(/^\s*\(/, false)) return "variableName.function";
    return "variableName";
  }

  if (stream.match(operador)) return "operator";
  if (stream.match(/^[()[\]{}]/)) return "bracket";
  if (stream.match(/^[,:]/)) return "punctuation";

  stream.next();
  return null;
}

function tokenBloco(stream: StringStream, estado: Estado): string {
  if (stream.skipTo("*/")) {
    stream.match("*/");
    estado.modo = "codigo";
  } else {
    stream.skipToEnd();
  }
  return "blockComment";
}

function tokenCru(stream: StringStream, estado: Estado): string {
  if (stream.skipTo("`")) {
    stream.next();
    estado.modo = "codigo";
  } else {
    stream.skipToEnd();
  }
  return "string";
}

export const gambiarraScript = StreamLanguage.define<Estado>({
  name: "gambiarrascript",
  startState: () => ({ modo: "codigo", interp: [], defineFuncao: false }),
  copyState: (e) => ({ ...e, interp: [...e.interp] }),
  token(stream, estado) {
    switch (estado.modo) {
      case "texto":
        return tokenTexto(stream, estado);
      case "cru":
        return tokenCru(stream, estado);
      case "bloco":
        return tokenBloco(stream, estado);
      default:
        return tokenCodigo(stream, estado);
    }
  },
  languageData: {
    commentTokens: { line: "#", block: { open: "/*", close: "*/" } },
  },
});
