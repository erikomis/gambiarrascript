import "./global.css";
import type { ReactNode } from "react";
import { RootProvider } from "fumadocs-ui/provider/next";

export const metadata = {
  title: "GambiarraScript — a linguagem do jeitinho brasileiro",
  description:
    "Documentacao e playground do GambiarraScript: a linguagem onde voce fecha bloco com `acabou_finalmente`.",
};

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <html lang="pt-BR" suppressHydrationWarning>
      <body>
        <RootProvider>{children}</RootProvider>
      </body>
    </html>
  );
}