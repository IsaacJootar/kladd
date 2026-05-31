"use client";

import { FormEvent, ReactNode, useEffect, useMemo, useState } from "react";

type Mode = "register" | "login";

type User = {
  id: string;
  name: string;
  email: string;
  phone?: string;
  account_type: string;
  verification_status: string;
  created_at: string;
};

type LoginResponse = {
  access_token: string;
  token_type: string;
  expires_at: string;
  user: User;
};

type PinResponse = {
  user_id: string;
  security_pin_set: boolean;
  security_pin_set_at: string;
};

type EvidenceItem = {
  id: string;
  category: string;
  display_name: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  status: string;
  uploaded_at: string;
};

type EvidenceListResponse = {
  items: EvidenceItem[];
};

type TruthDefinition = {
  id: string;
  truth_key: string;
  category: string;
  return_type: string;
  sensitivity: string;
  validity_days: number;
  derivation_rule: string;
  required_evidence: string[];
  created_at: string;
};

type TruthDefinitionListResponse = {
  items: TruthDefinition[];
};

const navItems = ["Home", "My Records", "Requests", "Proofs", "Security"];

const emptyRegisterForm = {
  name: "",
  email: "",
  phone: "",
  password: "",
  account_type: "individual",
};

const emptyLoginForm = {
  email: "",
  password: "",
};

const emptyEvidenceForm = {
  category: "passport",
  displayName: "",
  file: null as File | null,
};

export default function Home() {
  const [mode, setMode] = useState<Mode>("register");
  const [registerForm, setRegisterForm] = useState(emptyRegisterForm);
  const [loginForm, setLoginForm] = useState(emptyLoginForm);
  const [securityPIN, setSecurityPIN] = useState("");
  const [token, setToken] = useState(() =>
    readSessionValue("kladd_access_token"),
  );
  const [tokenExpiry, setTokenExpiry] = useState(() =>
    readSessionValue("kladd_token_expiry"),
  );
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [pinState, setPinState] = useState<PinResponse | null>(null);
  const [evidenceItems, setEvidenceItems] = useState<EvidenceItem[]>([]);
  const [truthDefinitions, setTruthDefinitions] = useState<TruthDefinition[]>([]);
  const [evidenceForm, setEvidenceForm] = useState(emptyEvidenceForm);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const signedIn = Boolean(token && currentUser);

  const statusCards = useMemo(
    () => [
      {
        label: "Identity Status",
        value: currentUser?.verification_status ?? "Not started",
      },
      { label: "Pending Requests", value: "0" },
      { label: "Supported Proofs", value: String(truthDefinitions.length) },
      {
        label: "My Evidence",
        value: String(evidenceItems.length),
      },
      { label: "Security PIN", value: pinState?.security_pin_set ? "Set" : "Not set" },
    ],
    [
      currentUser?.verification_status,
      truthDefinitions.length,
      evidenceItems.length,
      pinState?.security_pin_set,
    ],
  );

  useEffect(() => {
    if (!token) {
      return;
    }

    let ignore = false;
    Promise.all([
      apiRequest<User>("/account/me", {
        method: "GET",
        token,
      }),
      loadEvidenceItems(token),
      loadTruthDefinitions(token),
    ])
      .then(([user, evidence, truths]) => {
        if (!ignore) {
          setCurrentUser(user);
          setEvidenceItems(evidence);
          setTruthDefinitions(truths);
        }
      })
      .catch(() => {
        if (!ignore) {
          clearAuthStorage();
          setToken("");
          setTokenExpiry("");
          setCurrentUser(null);
          setPinState(null);
          setEvidenceItems([]);
          setTruthDefinitions([]);
        }
      });

    return () => {
      ignore = true;
    };
  }, [token]);

  async function handleRegister(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    clearMessages();

    try {
      await apiRequest<User>("/users", {
        method: "POST",
        body: JSON.stringify(registerForm),
      });

      const login = await loginWith(registerForm.email, registerForm.password);
      setCurrentUser(login.user);
      setMode("login");
      setRegisterForm(emptyRegisterForm);
      setNotice("Account created. You are signed in.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    clearMessages();

    try {
      const login = await loginWith(loginForm.email, loginForm.password);
      setCurrentUser(login.user);
      setLoginForm(emptyLoginForm);
      setNotice("Signed in. Your proof workspace is ready.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleSetPIN(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      setError("Please sign in before setting a Security PIN.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const result = await apiRequest<PinResponse>("/account/security-pin", {
        method: "POST",
        token,
        body: JSON.stringify({ security_pin: securityPIN }),
      });
      setPinState(result);
      setSecurityPIN("");
      setNotice("Security PIN set. Future claim approvals will require it.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleEvidenceUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      setError("Please sign in before adding evidence.");
      return;
    }

    if (!evidenceForm.file) {
      setError("Choose an evidence file first.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const formData = new FormData();
      formData.set("category", evidenceForm.category);
      formData.set("display_name", evidenceForm.displayName);
      formData.set("file", evidenceForm.file);

      const item = await apiMultipartRequest<EvidenceItem>("/evidence-items", {
        token,
        body: formData,
      });
      setEvidenceItems((items) => [item, ...items]);
      setEvidenceForm(emptyEvidenceForm);
      setNotice("Evidence record added.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function loginWith(email: string, password: string) {
    const login = await apiRequest<LoginResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    setToken(login.access_token);
    setTokenExpiry(login.expires_at);
    window.sessionStorage.setItem("kladd_access_token", login.access_token);
    window.sessionStorage.setItem("kladd_token_expiry", login.expires_at);
    return login;
  }

  function signOut() {
    setToken("");
    setTokenExpiry("");
    setCurrentUser(null);
    setPinState(null);
    setEvidenceItems([]);
    setTruthDefinitions([]);
    setEvidenceForm(emptyEvidenceForm);
    clearAuthStorage();
    setNotice("Signed out.");
    setError("");
  }

  function clearMessages() {
    setNotice("");
    setError("");
  }

  return (
    <main className="min-h-screen bg-[#eef3f8] text-slate-950">
      <div className="mx-auto flex min-h-screen w-full max-w-7xl flex-col px-4 py-4 sm:px-6 lg:px-8">
        <header className="border-b border-slate-200/80 pb-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-sm font-semibold text-indigo-700">
                Verify once. Prove everywhere.
              </p>
              <h1 className="mt-1 text-3xl font-semibold tracking-normal text-slate-950">
                Kladd
              </h1>
            </div>

            <nav className="flex flex-wrap gap-2" aria-label="Main navigation">
              {navItems.map((item) => (
                <span
                  key={item}
                  className="rounded-md border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 shadow-sm"
                >
                  {item}
                </span>
              ))}
            </nav>
          </div>
        </header>

        <section className="grid flex-1 gap-5 py-5 lg:grid-cols-[minmax(0,1fr)_390px]">
          <div className="space-y-5">
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div>
                  <p className="text-sm font-semibold text-emerald-700">
                    Account workspace
                  </p>
                  <h2 className="mt-1 text-2xl font-semibold tracking-normal text-slate-950">
                    Control your proofs
                  </h2>
                  <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
                    Create your account, sign in, and prepare the Security PIN
                    that protects future claim approvals.
                  </p>
                </div>

                {signedIn ? (
                  <button
                    type="button"
                    onClick={signOut}
                    className="h-10 rounded-md border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 shadow-sm transition hover:border-slate-400 hover:bg-slate-50"
                  >
                    Sign out
                  </button>
                ) : null}
              </div>

              <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                {statusCards.map((card) => (
                  <article
                    key={card.label}
                    className="min-h-28 rounded-lg border border-slate-200 bg-[#f9fbfd] p-4"
                  >
                    <p className="text-sm font-medium text-slate-500">
                      {card.label}
                    </p>
                    <p className="mt-3 text-2xl font-semibold capitalize text-slate-950">
                      {card.value}
                    </p>
                  </article>
                ))}
              </div>
            </section>

            {signedIn && currentUser ? (
              <>
                <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                  <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        Current account
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        {currentUser.name}
                      </h2>
                    </div>
                    <span className="w-fit rounded-md bg-emerald-50 px-3 py-2 text-sm font-semibold capitalize text-emerald-800">
                      {currentUser.verification_status}
                    </span>
                  </div>

                  <dl className="mt-5 grid gap-3 sm:grid-cols-2">
                    <ProfileField label="Email" value={currentUser.email} />
                    <ProfileField
                      label="Phone"
                      value={currentUser.phone || "Not added"}
                    />
                    <ProfileField
                      label="Account type"
                      value={currentUser.account_type}
                    />
                    <ProfileField
                      label="Token expires"
                      value={formatDateTime(tokenExpiry)}
                    />
                  </dl>
                </section>

                <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                  <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        My Records
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        Evidence vault
                      </h2>
                    </div>
                    <span className="w-fit rounded-md bg-indigo-50 px-3 py-2 text-sm font-semibold text-indigo-800">
                      {evidenceItems.length} records
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    {evidenceItems.length > 0 ? (
                      evidenceItems.map((item) => (
                        <EvidenceCard key={item.id} item={item} />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500 md:col-span-2">
                        No records yet.
                      </div>
                    )}
                  </div>
                </section>

                <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                  <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        Proofs
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        Supported truths
                      </h2>
                    </div>
                    <span className="w-fit rounded-md bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-800">
                      {truthDefinitions.length} definitions
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    {truthDefinitions.length > 0 ? (
                      truthDefinitions.map((definition) => (
                        <TruthDefinitionCard
                          key={definition.id}
                          definition={definition}
                        />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500 md:col-span-2">
                        No truth definitions loaded.
                      </div>
                    )}
                  </div>
                </section>
              </>
            ) : null}
          </div>

          <aside className="space-y-5">
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="grid grid-cols-2 rounded-lg bg-slate-100 p-1">
                <button
                  type="button"
                  onClick={() => setMode("register")}
                  className={modeButtonClass(mode === "register")}
                >
                  Register
                </button>
                <button
                  type="button"
                  onClick={() => setMode("login")}
                  className={modeButtonClass(mode === "login")}
                >
                  Login
                </button>
              </div>

              {mode === "register" ? (
                <form className="mt-5 space-y-4" onSubmit={handleRegister}>
                  <TextInput
                    label="Full name"
                    value={registerForm.name}
                    onChange={(value) =>
                      setRegisterForm((form) => ({ ...form, name: value }))
                    }
                    required
                  />
                  <TextInput
                    label="Email"
                    type="email"
                    value={registerForm.email}
                    onChange={(value) =>
                      setRegisterForm((form) => ({ ...form, email: value }))
                    }
                    required
                  />
                  <TextInput
                    label="Phone"
                    value={registerForm.phone}
                    onChange={(value) =>
                      setRegisterForm((form) => ({ ...form, phone: value }))
                    }
                  />
                  <TextInput
                    label="Password"
                    type="password"
                    value={registerForm.password}
                    onChange={(value) =>
                      setRegisterForm((form) => ({ ...form, password: value }))
                    }
                    minLength={8}
                    required
                  />
                  <label className="block">
                    <span className="text-sm font-semibold text-slate-700">
                      Account type
                    </span>
                    <select
                      value={registerForm.account_type}
                      onChange={(event) =>
                        setRegisterForm((form) => ({
                          ...form,
                          account_type: event.target.value,
                        }))
                      }
                      className="mt-2 h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-950 outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100"
                    >
                      <option value="individual">Individual</option>
                      <option value="business">Business</option>
                    </select>
                  </label>
                  <SubmitButton disabled={isSubmitting}>
                    Create account
                  </SubmitButton>
                </form>
              ) : (
                <form className="mt-5 space-y-4" onSubmit={handleLogin}>
                  <TextInput
                    label="Email"
                    type="email"
                    value={loginForm.email}
                    onChange={(value) =>
                      setLoginForm((form) => ({ ...form, email: value }))
                    }
                    required
                  />
                  <TextInput
                    label="Password"
                    type="password"
                    value={loginForm.password}
                    onChange={(value) =>
                      setLoginForm((form) => ({ ...form, password: value }))
                    }
                    required
                  />
                  <SubmitButton disabled={isSubmitting}>Sign in</SubmitButton>
                </form>
              )}
            </section>

            <section className="rounded-lg border border-indigo-100 bg-[#f8f7ff] p-5 shadow-sm">
              <div>
                <p className="text-sm font-semibold text-indigo-700">
                  Security
                </p>
                <h2 className="mt-1 text-lg font-semibold tracking-normal">
                  Security PIN
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  This PIN will be required before any approved proof can be
                  released.
                </p>
              </div>

              <form className="mt-5 space-y-4" onSubmit={handleSetPIN}>
                <TextInput
                  label="4-6 digit PIN"
                  type="password"
                  inputMode="numeric"
                  value={securityPIN}
                  onChange={setSecurityPIN}
                  minLength={4}
                  maxLength={6}
                  disabled={!signedIn}
                  required
                />
                <SubmitButton disabled={!signedIn || isSubmitting}>
                  Set Security PIN
                </SubmitButton>
              </form>
            </section>

            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div>
                <p className="text-sm font-semibold text-slate-500">
                  My Records
                </p>
                <h2 className="mt-1 text-lg font-semibold tracking-normal">
                  Add evidence
                </h2>
              </div>

              <form className="mt-5 space-y-4" onSubmit={handleEvidenceUpload}>
                <label className="block">
                  <span className="text-sm font-semibold text-slate-700">
                    Category
                  </span>
                  <select
                    value={evidenceForm.category}
                    onChange={(event) =>
                      setEvidenceForm((form) => ({
                        ...form,
                        category: event.target.value,
                      }))
                    }
                    disabled={!signedIn}
                    className="mt-2 h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-950 outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
                  >
                    <option value="passport">Passport</option>
                    <option value="degree_certificate">Degree certificate</option>
                    <option value="business_registration">Business registration</option>
                    <option value="utility_bill">Utility bill</option>
                    <option value="license">License</option>
                    <option value="tax_document">Tax document</option>
                  </select>
                </label>

                <TextInput
                  label="Display name"
                  value={evidenceForm.displayName}
                  onChange={(value) =>
                    setEvidenceForm((form) => ({
                      ...form,
                      displayName: value,
                    }))
                  }
                  disabled={!signedIn}
                />

                <label className="block">
                  <span className="text-sm font-semibold text-slate-700">
                    File
                  </span>
                  <input
                    type="file"
                    onChange={(event) =>
                      setEvidenceForm((form) => ({
                        ...form,
                        file: event.target.files?.[0] ?? null,
                      }))
                    }
                    disabled={!signedIn}
                    required
                    className="mt-2 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-950 outline-none transition file:mr-3 file:rounded-md file:border-0 file:bg-indigo-50 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-indigo-800 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
                  />
                </label>

                <SubmitButton disabled={!signedIn || isSubmitting}>
                  Add evidence
                </SubmitButton>
              </form>
            </section>

            {(notice || error) && (
              <section
                className={`rounded-lg border p-4 text-sm leading-6 shadow-sm ${
                  error
                    ? "border-red-200 bg-red-50 text-red-800"
                    : "border-emerald-200 bg-emerald-50 text-emerald-800"
                }`}
              >
                {error || notice}
              </section>
            )}
          </aside>
        </section>
      </div>
    </main>
  );
}

function ProfileField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <dt className="text-sm font-medium text-slate-500">{label}</dt>
      <dd className="mt-2 break-words text-sm font-semibold text-slate-950">
        {value}
      </dd>
    </div>
  );
}

function EvidenceCard({ item }: { item: EvidenceItem }) {
  return (
    <article className="min-h-40 rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold capitalize text-slate-950">
            {item.display_name}
          </p>
          <p className="mt-1 text-sm text-slate-500">
            {formatCategory(item.category)}
          </p>
        </div>
        <span className="rounded-md bg-amber-50 px-2.5 py-1 text-xs font-semibold capitalize text-amber-800">
          {formatCategory(item.status)}
        </span>
      </div>

      <dl className="mt-5 space-y-2 text-sm">
        <div className="flex justify-between gap-3">
          <dt className="text-slate-500">File</dt>
          <dd className="break-all text-right font-medium text-slate-800">
            {item.file_name}
          </dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-slate-500">Size</dt>
          <dd className="font-medium text-slate-800">
            {formatBytes(item.size_bytes)}
          </dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-slate-500">Uploaded</dt>
          <dd className="font-medium text-slate-800">
            {formatDateTime(item.uploaded_at)}
          </dd>
        </div>
      </dl>
    </article>
  );
}

function TruthDefinitionCard({
  definition,
}: {
  definition: TruthDefinition;
}) {
  return (
    <article className="min-h-44 rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="break-words text-sm font-semibold text-slate-950">
            {definition.truth_key}
          </p>
          <p className="mt-1 text-sm capitalize text-slate-500">
            {formatCategory(definition.category)}
          </p>
        </div>
        <span className={sensitivityClass(definition.sensitivity)}>
          {definition.sensitivity}
        </span>
      </div>

      <dl className="mt-5 space-y-2 text-sm">
        <div className="flex justify-between gap-3">
          <dt className="text-slate-500">Return</dt>
          <dd className="font-medium capitalize text-slate-800">
            {formatCategory(definition.return_type)}
          </dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-slate-500">Valid for</dt>
          <dd className="font-medium text-slate-800">
            {definition.validity_days} days
          </dd>
        </div>
        <div>
          <dt className="text-slate-500">Evidence</dt>
          <dd className="mt-1 text-sm font-medium capitalize text-slate-800">
            {definition.required_evidence.map(formatCategory).join(", ")}
          </dd>
        </div>
      </dl>
    </article>
  );
}

function TextInput({
  label,
  value,
  onChange,
  type = "text",
  inputMode,
  minLength,
  maxLength,
  disabled,
  required,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  inputMode?: "numeric";
  minLength?: number;
  maxLength?: number;
  disabled?: boolean;
  required?: boolean;
}) {
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <input
        type={type}
        inputMode={inputMode}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        minLength={minLength}
        maxLength={maxLength}
        disabled={disabled}
        required={required}
        className="mt-2 h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-950 outline-none transition placeholder:text-slate-400 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
      />
    </label>
  );
}

function SubmitButton({
  children,
  disabled,
}: {
  children: ReactNode;
  disabled?: boolean;
}) {
  return (
    <button
      type="submit"
      disabled={disabled}
      className="h-11 w-full rounded-md bg-indigo-700 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-indigo-800 disabled:cursor-not-allowed disabled:bg-slate-300 disabled:text-slate-600"
    >
      {children}
    </button>
  );
}

function readSessionValue(key: string) {
  if (typeof window === "undefined") {
    return "";
  }

  return window.sessionStorage.getItem(key) ?? "";
}

function clearAuthStorage() {
  window.sessionStorage.removeItem("kladd_access_token");
  window.sessionStorage.removeItem("kladd_token_expiry");
}

function modeButtonClass(active: boolean) {
  return `h-10 rounded-md text-sm font-semibold transition ${
    active
      ? "bg-white text-indigo-800 shadow-sm"
      : "text-slate-600 hover:text-slate-950"
  }`;
}

async function apiRequest<T>(
  path: string,
  options: {
    method: "GET" | "POST";
    body?: string;
    token?: string;
  },
): Promise<T> {
  const headers = new Headers();
  headers.set("content-type", "application/json");
  if (options.token) {
    headers.set("authorization", `Bearer ${options.token}`);
  }

  const response = await fetch(`/api/kladd${path}`, {
    method: options.method,
    headers,
    body: options.body,
  });

  const text = await response.text();
  const payload = parseJSON(text);
  if (!response.ok) {
    throw new Error(payload?.message ?? "Request failed.");
  }

  return payload as T;
}

async function apiMultipartRequest<T>(
  path: string,
  options: {
    body: FormData;
    token?: string;
  },
): Promise<T> {
  const headers = new Headers();
  if (options.token) {
    headers.set("authorization", `Bearer ${options.token}`);
  }

  const response = await fetch(`/api/kladd${path}`, {
    method: "POST",
    headers,
    body: options.body,
  });

  const text = await response.text();
  const payload = parseJSON(text);
  if (!response.ok) {
    throw new Error(payload?.message ?? "Request failed.");
  }

  return payload as T;
}

async function loadEvidenceItems(accessToken: string) {
  const response = await apiRequest<EvidenceListResponse>("/evidence-items", {
    method: "GET",
    token: accessToken,
  });

  return response.items;
}

async function loadTruthDefinitions(accessToken: string) {
  const response = await apiRequest<TruthDefinitionListResponse>(
    "/truth-definitions",
    {
      method: "GET",
      token: accessToken,
    },
  );

  return response.items;
}

function parseJSON(text: string) {
  if (!text) {
    return null;
  }

  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

function readError(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }

  return "Something went wrong.";
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

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B";
  }

  if (value < 1024) {
    return `${value} B`;
  }

  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }

  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatCategory(value: string) {
  return value.replaceAll("_", " ");
}

function sensitivityClass(value: string) {
  const base =
    "rounded-md px-2.5 py-1 text-xs font-semibold capitalize";
  if (value === "low") {
    return `${base} bg-emerald-50 text-emerald-800`;
  }
  if (value === "medium") {
    return `${base} bg-amber-50 text-amber-800`;
  }
  if (value === "high") {
    return `${base} bg-red-50 text-red-800`;
  }

  return `${base} bg-slate-100 text-slate-700`;
}
