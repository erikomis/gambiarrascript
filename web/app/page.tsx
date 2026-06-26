import Link from "next/link";
import { buttonVariants } from "fumadocs-ui/components/ui/button";
import { HomeLayout } from "fumadocs-ui/layouts/home";
import { navDefaults, navLinks } from "@/lib/nav";

export default function HomePage() {
  return (
    <HomeLayout nav={navDefaults}>
      <main className="flex min-h-[calc(100vh-4rem)] flex-col items-center gap-8 px-6 pt-24 pb-20 text-center">
        <div className="flex flex-col items-center gap-4">
          <span className="rounded-full border border-fd-foreground/15 bg-fd-muted px-3 py-1 text-sm text-fd-muted-foreground">
            🇧🇷 linguagem do jeitinho brasileiro
          </span>
          <h1 className="text-5xl font-semibold tracking-tight md:text-7xl">
            GambiarraScript
          </h1>
          <p className="max-w-2xl text-lg text-fd-muted-foreground">
            A linguagem onde voce nao fecha bloco com <code>{"}"}</code> nem com{" "}
            <code>end</code> — voce fecha com <code>acabou_finalmente</code>.
            Escrita em Go.
          </p>
        </div>

        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link href="/docs" className={buttonVariants({ color: "primary" })}>
            Ler a documentacao
          </Link>
          <Link
            href="/playground"
            className={buttonVariants({ color: "outline" })}
          >
            Abrir o playground
          </Link>
        </div>

        <pre className="mt-4 max-w-xl overflow-auto rounded-lg border border-fd-foreground/10 bg-fd-muted p-4 text-left text-sm">
          <code>{`mostra "Salve, tropa!"

bota nome = "Erik"
bota idade = 25

se_colar idade >= 18
    mostra nome + " pode entrar"
se_nao_colar
    mostra "volta daqui a pouco"
acabou_finalmente`}</code>
        </pre>

        <nav className="flex flex-wrap items-center justify-center gap-4 text-sm text-fd-muted-foreground">
          {navLinks.map((l) => (
            <Link key={l.url} href={l.url}>
              {l.text}
            </Link>
          ))}
        </nav>
      </main>
    </HomeLayout>
  );
}