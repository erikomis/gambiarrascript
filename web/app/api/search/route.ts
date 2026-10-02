import { source } from "@/lib/source";
import { createFromSource } from "fumadocs-core/search/server";

// indice de busca gerado no build (static export) — o navegador baixa o
// JSON e busca localmente, sem servidor.
export const revalidate = false;
export const { staticGET: GET } = createFromSource(source);
