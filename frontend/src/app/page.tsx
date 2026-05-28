const userCards = [
  { label: "Identity Status", value: "Not started" },
  { label: "Pending Requests", value: "0" },
  { label: "Active Proofs", value: "0" },
  { label: "Recent Activity", value: "None yet" },
];

const navigation = ["Home", "My Records", "Requests", "Proofs", "Security"];

export default function Home() {
  return (
    <main className="min-h-screen bg-[#f7f9fc] text-slate-950">
      <div className="mx-auto flex min-h-screen w-full max-w-6xl flex-col px-5 py-6 sm:px-8">
        <header className="flex flex-col gap-5 border-b border-slate-200 pb-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-medium text-indigo-700">
              Verify once. Prove everywhere.
            </p>
            <h1 className="mt-2 text-3xl font-semibold tracking-normal text-slate-950">
              Kladd
            </h1>
          </div>
          <nav className="flex flex-wrap gap-2" aria-label="Main navigation">
            {navigation.map((item) => (
              <span
                key={item}
                className="rounded-md border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 shadow-sm"
              >
                {item}
              </span>
            ))}
          </nav>
        </header>

        <section className="grid flex-1 gap-6 py-8 lg:grid-cols-[1.1fr_0.9fr]">
          <div className="space-y-6">
            <div>
              <h2 className="text-2xl font-semibold tracking-normal">
                Proof dashboard
              </h2>
              <p className="mt-2 max-w-2xl text-base leading-7 text-slate-600">
                A calm workspace for records, requests, approvals, and proof
                history.
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              {userCards.map((card) => (
                <article
                  key={card.label}
                  className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
                >
                  <p className="text-sm font-medium text-slate-500">
                    {card.label}
                  </p>
                  <p className="mt-3 text-2xl font-semibold text-slate-950">
                    {card.value}
                  </p>
                </article>
              ))}
            </div>
          </div>

          <aside className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <h2 className="text-lg font-semibold tracking-normal">
              Next foundation
            </h2>
            <div className="mt-5 space-y-4">
              <div>
                <p className="text-sm font-medium text-slate-950">
                  Frontend stack
                </p>
                <p className="mt-1 text-sm leading-6 text-slate-600">
                  TypeScript, Next.js, Tailwind CSS, and shadcn/ui-ready
                  structure.
                </p>
              </div>
              <div>
                <p className="text-sm font-medium text-slate-950">
                  Backend stack
                </p>
                <p className="mt-1 text-sm leading-6 text-slate-600">
                  Go API scaffold prepared for the documented Kladd modules.
                </p>
              </div>
              <div>
                <p className="text-sm font-medium text-slate-950">
                  Product guardrail
                </p>
                <p className="mt-1 text-sm leading-6 text-slate-600">
                  Raw documents and sensitive identity anchors are not exposed
                  by this scaffold.
                </p>
              </div>
            </div>
          </aside>
        </section>
      </div>
    </main>
  );
}
