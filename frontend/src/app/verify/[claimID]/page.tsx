type VerifyPageProps = {
  params: Promise<{
    claimID: string;
  }>;
};

type VerificationClaim = {
  id: string;
  organization: {
    name: string;
    organization_type: string;
    verification_status: string;
  };
  purpose: string;
  approved_truths?: string[];
  status: string;
  issued_at: string;
  expires_at: string;
  revoked_at?: string;
  details_visible: boolean;
};

const apiBaseURL = process.env.KLADD_API_BASE_URL ?? "http://localhost:8080";

export default async function VerifyPage({ params }: VerifyPageProps) {
  const { claimID } = await params;
  const claim = await loadClaimStatus(claimID);

  if (!claim) {
    return (
      <main className="min-h-screen bg-[#eef3f8] px-4 py-6 text-slate-950 sm:px-6 lg:px-8">
        <section className="mx-auto flex min-h-[calc(100vh-3rem)] max-w-3xl items-center">
          <div className="w-full rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-sm font-semibold text-red-700">
              Verification unavailable
            </p>
            <h1 className="mt-2 text-2xl font-semibold tracking-normal">
              Proof not found
            </h1>
            <p className="mt-3 text-sm leading-6 text-slate-600">
              This proof link may be incorrect or no longer available.
            </p>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-[#eef3f8] px-4 py-6 text-slate-950 sm:px-6 lg:px-8">
      <section className="mx-auto max-w-4xl">
        <header className="border-b border-slate-200/80 pb-5">
          <p className="text-sm font-semibold text-indigo-700">
            Kladd verification
          </p>
          <h1 className="mt-2 text-3xl font-semibold tracking-normal">
            Proof status
          </h1>
        </header>

        <section className="mt-5 rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
            <div>
              <p className="text-sm font-semibold text-slate-500">
                Requester
              </p>
              <h2 className="mt-1 text-2xl font-semibold tracking-normal">
                {claim.organization.name}
              </h2>
              <p className="mt-2 text-sm leading-6 text-slate-600">
                {claim.purpose}
              </p>
            </div>
            <span className={statusClass(claim.status)}>
              {formatClaimStatus(claim.status)}
            </span>
          </div>

          <dl className="mt-5 grid gap-3 sm:grid-cols-2">
            <VerificationField label="Issued" value={formatDateTime(claim.issued_at)} />
            <VerificationField label="Expires" value={formatDateTime(claim.expires_at)} />
            <VerificationField
              label="Requester type"
              value={formatCategory(claim.organization.organization_type)}
            />
            <VerificationField
              label="Requester status"
              value={formatCategory(claim.organization.verification_status)}
            />
          </dl>

          <section className="mt-5 border-t border-slate-200 pt-5">
            <p className="text-sm font-semibold text-slate-500">Proofs</p>
            {claim.details_visible ? (
              <div className="mt-3 flex flex-wrap gap-2">
                {(claim.approved_truths ?? []).map((truth) => (
                  <span
                    key={truth}
                    className="rounded-md border border-slate-200 bg-[#f9fbfd] px-3 py-2 text-sm font-semibold text-slate-800"
                  >
                    {formatProofName(truth)}
                  </span>
                ))}
              </div>
            ) : (
              <p className="mt-3 rounded-md bg-[#f9fbfd] px-3 py-2 text-sm font-medium text-slate-600">
                Proof details are hidden because this claim is {formatClaimStatus(claim.status).toLowerCase()}.
              </p>
            )}
          </section>
        </section>
      </section>
    </main>
  );
}

async function loadClaimStatus(claimID: string) {
  const response = await fetch(
    new URL(`/api/claims/${claimID}/status`, apiBaseURL),
    {
      cache: "no-store",
    },
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error("Unable to load proof verification.");
  }

  return (await response.json()) as VerificationClaim;
}

function VerificationField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <dt className="text-sm font-medium text-slate-500">{label}</dt>
      <dd className="mt-2 break-words text-sm font-semibold text-slate-950">
        {value}
      </dd>
    </div>
  );
}

function formatDateTime(value: string) {
  if (!value) {
    return "Not available";
  }

  return new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatCategory(value: string) {
  return value.replaceAll("_", " ");
}

function formatProofName(value: string) {
  const labels: Record<string, string> = {
    identity_verified: "Identity proof",
    age_over_18: "Age check",
    address_verified: "Address proof",
    degree_verified: "Education proof",
    business_registered: "Business proof",
    license_active: "License proof",
  };

  return labels[value] ?? formatCategory(value);
}

function formatClaimStatus(value: string) {
  if (value === "active") {
    return "Active";
  }
  if (value === "expired") {
    return "Expired";
  }
  if (value === "revoked") {
    return "Revoked";
  }

  return formatCategory(value);
}

function statusClass(value: string) {
  const base = "w-fit rounded-md px-3 py-2 text-sm font-semibold";
  if (value === "active") {
    return `${base} bg-emerald-50 text-emerald-800`;
  }
  if (value === "revoked") {
    return `${base} bg-red-50 text-red-800`;
  }

  return `${base} bg-slate-100 text-slate-700`;
}
