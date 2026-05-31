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
      { label: "Active Proofs", value: "0" },
      {
        label: "Security PIN",
        value: pinState?.security_pin_set ? "Set" : "Not set",
      },
    ],
    [currentUser?.verification_status, pinState?.security_pin_set],
  );

  useEffect(() => {
    if (!token) {
      return;
    }

    let ignore = false;
    apiRequest<User>("/account/me", {
      method: "GET",
      token,
    })
      .then((user) => {
        if (!ignore) {
          setCurrentUser(user);
        }
      })
      .catch(() => {
        if (!ignore) {
          clearAuthStorage();
          setToken("");
          setTokenExpiry("");
          setCurrentUser(null);
          setPinState(null);
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
