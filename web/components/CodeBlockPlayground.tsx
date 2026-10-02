"use client";

// CodeBlock do fumadocs com um link extra "Rodar no playground" ao lado do
// botao de copiar. Precisa ser client component so porque o `Actions` do
// CodeBlock eh uma funcao (nao atravessa a fronteira server -> client); o href
// chega pronto do build, entao o link funciona ate sem JS e com clique do meio.
import type { ComponentProps } from "react";
import { CodeBlock } from "fumadocs-ui/components/codeblock";
import { buttonVariants } from "fumadocs-ui/components/ui/button";

type Props = ComponentProps<typeof CodeBlock> & {
  href: string;
  rotulo: string;
  dica: string;
};

export function CodeBlockPlayground({ href, rotulo, dica, ...props }: Props) {
  return (
    <CodeBlock
      {...props}
      Actions={({ className, children }) => (
        <div className={`flex items-center gap-0.5 ${className ?? ""}`}>
          <a
            href={href}
            title={dica}
            aria-label={rotulo}
            className={buttonVariants({
              size: "sm",
              className:
                "py-1 hover:bg-fd-accent hover:text-fd-accent-foreground [&_svg]:size-3.5",
            })}
          >
            <svg
              viewBox="0 0 24 24"
              fill="currentColor"
              aria-hidden="true"
            >
              <path d="M7 4.5v15a1 1 0 0 0 1.53.85l12-7.5a1 1 0 0 0 0-1.7l-12-7.5A1 1 0 0 0 7 4.5Z" />
            </svg>
            {/* no celular fica so o icone, pra nao cobrir o codigo */}
            <span className="hidden sm:inline">{rotulo}</span>
          </a>
          {children}
        </div>
      )}
    />
  );
}
