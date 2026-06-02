"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";

type VerificationClaim = {
  id: string;
};

export default function VerifyPINPage() {
  const router = useRouter();
  const [exchangePIN, setExchangePIN] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setIsSubmitting(true);

    try {
      const response = await fetch("/api/kladd/exchange-pins/resolve", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ exchange_pin: exchangePIN.trim() }),
      });

      if (!response.ok) {
        setError("Exchange PIN was not found or has expired.");
        return;
      }

      const claim = (await response.json()) as VerificationClaim;
      router.push(`/verify/${claim.id}`);
    } catch {
      setError("Unable to verify exchange PIN.");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="min-h-screen bg-[#eef3f8] px-4 py-6 text-slate-950 sm:px-6 lg:px-8">
      <section className="mx-auto flex min-h-[calc(100vh-3rem)] max-w-3xl items-center">
        <div className="w-full rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-sm font-semibold text-indigo-700">
            Kladd verification
          </p>
          <h1 className="mt-2 text-3xl font-semibold tracking-normal">
            Enter exchange PIN
          </h1>
          <p className="mt-3 text-sm leading-6 text-slate-600">
            Use the temporary PIN shared by the proof owner to open the
            verification page.
          </p>

          <form onSubmit={handleSubmit} className="mt-6 grid gap-4">
            <label className="grid gap-2 text-sm font-semibold text-slate-700">
              Exchange PIN
              <input
                value={exchangePIN}
                onChange={(event) => setExchangePIN(event.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={8}
                placeholder="12345678"
                className="h-12 rounded-md border border-slate-300 bg-white px-3 font-mono text-lg font-semibold tracking-normal text-slate-950 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"
              />
            </label>

            {error ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm font-semibold text-red-700">
                {error}
              </p>
            ) : null}

            <button
              type="submit"
              disabled={isSubmitting || exchangePIN.trim().length < 6}
              className="h-12 rounded-md bg-slate-950 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300"
            >
              {isSubmitting ? "Checking..." : "Open verification"}
            </button>
          </form>
        </div>
      </section>
    </main>
  );
}
