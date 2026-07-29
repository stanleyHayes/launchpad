import type { Metadata } from "next";
import Link from "next/link";
import { Container } from "@launchpad/ui";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";

export const metadata: Metadata = {
  title: "LaunchPad — L’intégration des employés, orchestrée",
  description: "Créez des parcours guidés, automatisez la mise en place et mesurez le délai de productivité.",
  alternates: { canonical: "/fr", languages: { en: "/", fr: "/fr" } },
};

export default function FrenchHomePage() {
  return (
    <main className="min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-24 pt-40">
        <Container>
          <p className="lp-eyebrow">LaunchPad en français</p>
          <h1 className="mt-5 max-w-4xl text-6xl font-semibold tracking-tight">
            Chaque nouvelle recrue sait quoi faire, qui contacter et comment réussir.
          </h1>
          <p className="mt-6 max-w-2xl text-xl leading-8 text-[var(--lp-ink-muted)]">
            Concevez des parcours d’intégration cohérents pour les RH, l’informatique, les responsables et les employés.
          </p>
          <div className="mt-9 flex flex-wrap gap-3">
            <Link className="lp-btn lp-btn--primary" href="/signup">Commencer gratuitement</Link>
            <Link className="lp-btn lp-btn--secondary" href="/demo">Demander une démonstration</Link>
          </div>
          <div className="mt-16 grid gap-4 md:grid-cols-3">
            {[
              ["Parcours guidés", "Étapes, dépendances, approbations et rappels dans un flux clair."],
              ["Aide contextuelle", "Des réponses citées à partir de votre base de connaissances approuvée."],
              ["Progrès mesurable", "Repérez les retards, les abandons et les étapes qui accélèrent la réussite."],
            ].map(([title, body]) => (
              <article className="lp-card p-6" key={title}>
                <h2 className="text-xl font-semibold">{title}</h2>
                <p className="mt-3 leading-7 text-[var(--lp-ink-muted)]">{body}</p>
              </article>
            ))}
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
