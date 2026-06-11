"use client";

import { FormEvent, ReactNode, useEffect, useMemo, useState } from "react";
import Image from "next/image";
import QRCode from "qrcode";

type Mode = "register" | "login";
type OrganizationMode = "register" | "login";
type WorkspaceMode = "personal" | "organization";

type User = {
  id: string;
  name: string;
  email: string;
  phone?: string;
  account_type: string;
  verification_status: string;
  created_at: string;
};

type Organization = {
  id: string;
  name: string;
  organization_type: string;
  verification_status: string;
};

type LoginResponse = {
  access_token: string;
  token_type: string;
  expires_at: string;
  user: User;
};

type OrganizationLoginResponse = {
  access_token: string;
  token_type: string;
  expires_at: string;
  organization: Organization;
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

type ClaimRequest = {
  id: string;
  organization: Organization;
  purpose: string;
  requested_truths: string[];
  status: string;
  expires_at: string;
  created_at: string;
};

type ClaimRequestListResponse = {
  items: ClaimRequest[];
};

type ApprovalResponse = {
  consent_id: string;
  claim_id: string;
  claim_request: ClaimRequest;
  approved_at: string;
};

type Claim = {
  id: string;
  claim_request_id: string;
  organization: Organization;
  purpose: string;
  approved_truths?: string[];
  status: string;
  issued_at: string;
  expires_at: string;
  revoked_at?: string;
  details_visible: boolean;
};

type ClaimListResponse = {
  items: Claim[];
};

type WebhookEndpoint = {
  id: string;
  organization: Organization;
  url: string;
  status: string;
  created_at: string;
  updated_at: string;
};

type WebhookDelivery = {
  id: string;
  event_type: string;
  aggregate_id: string;
  organization_id: string;
  status: string;
  attempts: number;
  next_attempt_at?: string;
  delivered_at?: string;
  created_at: string;
  updated_at: string;
};

type WebhookDeliveryListResponse = {
  items: WebhookDelivery[];
};

type ExchangePIN = {
  claim_id: string;
  exchange_pin: string;
  expires_at: string;
};

type ActivityItem = {
  id: string;
  event_type: string;
  title: string;
  description: string;
  created_at: string;
};

type ActivityListResponse = {
  items: ActivityItem[];
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

type OrganizationRequestForm = {
  userEmail: string;
  purpose: string;
  requestedTruths: string[];
  durationDays: string;
};

type OrganizationRegisterForm = {
  name: string;
  email: string;
  password: string;
  organizationType: string;
};

type OrganizationLoginForm = {
  email: string;
  password: string;
};

type ProofReadiness = {
  key: string;
  label: string;
  category: string;
  requiredEvidence: string[];
  status: "ready" | "review" | "missing";
};

const navItems = [
  { label: "Home", targetID: "workspace-home", requiresAuth: false },
  { label: "My Records", targetID: "my-records", requiresAuth: true },
  { label: "Requests", targetID: "proof-requests", requiresAuth: true },
  { label: "Proofs", targetID: "active-proofs", requiresAuth: true },
  { label: "Security", targetID: "security", requiresAuth: false },
];

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

const emptyResetPINForm = {
  password: "",
  securityPIN: "",
};

const emptyEvidenceForm = {
  category: "passport",
  displayName: "",
  file: null as File | null,
};

const emptyOrganizationRequestForm: OrganizationRequestForm = {
  userEmail: "",
  purpose: "Account opening",
  requestedTruths: ["identity_verified"],
  durationDays: "30",
};

const emptyOrganizationRegisterForm: OrganizationRegisterForm = {
  name: "",
  email: "",
  password: "",
  organizationType: "organization",
};

const emptyOrganizationLoginForm: OrganizationLoginForm = {
  email: "",
  password: "",
};

export default function Home() {
  const [mode, setMode] = useState<Mode>("register");
  const [organizationMode, setOrganizationMode] =
    useState<OrganizationMode>("register");
  const [workspaceMode, setWorkspaceMode] =
    useState<WorkspaceMode>("personal");
  const [registerForm, setRegisterForm] = useState(emptyRegisterForm);
  const [loginForm, setLoginForm] = useState(emptyLoginForm);
  const [securityPIN, setSecurityPIN] = useState("");
  const [resetPINForm, setResetPINForm] = useState(emptyResetPINForm);
  const [token, setToken] = useState(() =>
    readSessionValue("kladd_access_token"),
  );
  const [tokenExpiry, setTokenExpiry] = useState(() =>
    readSessionValue("kladd_token_expiry"),
  );
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [evidenceItems, setEvidenceItems] = useState<EvidenceItem[]>([]);
  const [claimRequests, setClaimRequests] = useState<ClaimRequest[]>([]);
  const [claims, setClaims] = useState<Claim[]>([]);
  const [activityItems, setActivityItems] = useState<ActivityItem[]>([]);
  const [truthDefinitions, setTruthDefinitions] = useState<TruthDefinition[]>(
    [],
  );
  const [approvalPINs, setApprovalPINs] = useState<Record<string, string>>({});
  const [evidenceForm, setEvidenceForm] = useState(emptyEvidenceForm);
  const [organizationRequestForm, setOrganizationRequestForm] = useState(
    emptyOrganizationRequestForm,
  );
  const [organizationRegisterForm, setOrganizationRegisterForm] = useState(
    emptyOrganizationRegisterForm,
  );
  const [organizationLoginForm, setOrganizationLoginForm] = useState(
    emptyOrganizationLoginForm,
  );
  const [organizationToken, setOrganizationToken] = useState(() =>
    readSessionValue("kladd_organization_access_token"),
  );
  const [organizationTokenExpiry, setOrganizationTokenExpiry] = useState(() =>
    readSessionValue("kladd_organization_token_expiry"),
  );
  const [createdOrganizationRequest, setCreatedOrganizationRequest] =
    useState<ClaimRequest | null>(null);
  const [organizationProfile, setOrganizationProfile] =
    useState<Organization | null>(null);
  const [organizationClaimRequests, setOrganizationClaimRequests] = useState<
    ClaimRequest[]
  >([]);
  const [organizationClaims, setOrganizationClaims] = useState<Claim[]>([]);
  const [organizationWebhookEndpoint, setOrganizationWebhookEndpoint] =
    useState<WebhookEndpoint | null>(null);
  const [organizationWebhookURL, setOrganizationWebhookURL] = useState("");
  const [organizationWebhookDeliveries, setOrganizationWebhookDeliveries] =
    useState<WebhookDelivery[]>([]);
  const [organizationActivityItems, setOrganizationActivityItems] = useState<
    ActivityItem[]
  >([]);
  const [copiedClaimID, setCopiedClaimID] = useState("");
  const [claimQRCodes, setClaimQRCodes] = useState<Record<string, string>>({});
  const [claimExchangePINs, setClaimExchangePINs] = useState<
    Record<string, ExchangePIN>
  >({});
  const [copiedExchangePINClaimID, setCopiedExchangePINClaimID] = useState("");
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const signedIn = Boolean(token && currentUser);
  const organizationSignedIn = Boolean(organizationToken && organizationProfile);
  const pendingClaimRequests = useMemo(
    () =>
      claimRequests.filter((request) => request.status === "pending_approval"),
    [claimRequests],
  );
  const activeClaims = useMemo(
    () => claims.filter((claim) => claim.status === "active"),
    [claims],
  );
  const organizationPendingRequests = useMemo(
    () =>
      organizationClaimRequests.filter(
        (request) => request.status === "pending_approval",
      ),
    [organizationClaimRequests],
  );
  const organizationActiveClaims = useMemo(
    () => organizationClaims.filter((claim) => claim.status === "active"),
    [organizationClaims],
  );
  const organizationClosedClaims = useMemo(
    () => organizationClaims.filter((claim) => claim.status !== "active"),
    [organizationClaims],
  );
  const proofOptions = useMemo(() => {
    const definitions =
      truthDefinitions.length > 0
        ? truthDefinitions
        : [
            { truth_key: "identity_verified", category: "identity" },
            { truth_key: "address_verified", category: "address" },
            { truth_key: "degree_verified", category: "education" },
            { truth_key: "business_registered", category: "business" },
            { truth_key: "license_active", category: "licensing" },
            { truth_key: "age_over_18", category: "age" },
          ];

    return definitions.map((definition) => ({
      key: definition.truth_key,
      label: formatProofName(definition.truth_key),
      category: formatCategory(definition.category),
    }));
  }, [truthDefinitions]);
  const proofReadiness = useMemo<ProofReadiness[]>(
    () =>
      truthDefinitions.map((definition) => {
        const requiredEvidence = definition.required_evidence;
        const requiredRecords = requiredEvidence.map((category) =>
          evidenceItems.filter((item) => item.category === category),
        );
        const allVerified =
          requiredEvidence.length > 0 &&
          requiredRecords.every((records) =>
            records.some((record) => record.status === "verified"),
          );
        const allPresent =
          requiredEvidence.length > 0 &&
          requiredRecords.every((records) => records.length > 0);

        return {
          key: definition.truth_key,
          label: formatProofName(definition.truth_key),
          category: formatCategory(definition.category),
          requiredEvidence,
          status: allVerified ? "ready" : allPresent ? "review" : "missing",
        };
      }),
    [evidenceItems, truthDefinitions],
  );
  const readyProofCount = useMemo(
    () => proofReadiness.filter((proof) => proof.status === "ready").length,
    [proofReadiness],
  );

  const statusCards = useMemo(
    () => [
      {
        label: "Identity Status",
        value: currentUser?.verification_status ?? "Not started",
      },
      { label: "Pending Requests", value: String(pendingClaimRequests.length) },
      { label: "Active Proofs", value: String(activeClaims.length) },
      {
        label: "Recent Activity",
        value: activityItems.length > 0 ? String(activityItems.length) : "Ready",
      },
    ],
    [
      currentUser?.verification_status,
      pendingClaimRequests.length,
      activeClaims.length,
      activityItems.length,
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
      loadClaimRequests(token),
      loadClaims(token),
      loadActivityItems(token),
      loadTruthDefinitions(token),
    ])
      .then(([user, evidence, requests, loadedClaims, activity, definitions]) => {
        if (!ignore) {
          setCurrentUser(user);
          setEvidenceItems(evidence);
          setClaimRequests(requests);
          setClaims(loadedClaims);
          setActivityItems(activity);
          setTruthDefinitions(definitions);
        }
      })
      .catch(() => {
        if (!ignore) {
          clearAuthStorage();
          setToken("");
          setTokenExpiry("");
          setCurrentUser(null);
          setEvidenceItems([]);
          setClaimRequests([]);
          setClaims([]);
          setActivityItems([]);
          setTruthDefinitions([]);
          setApprovalPINs({});
        }
      });

    return () => {
      ignore = true;
    };
  }, [token]);

  useEffect(() => {
    if (!organizationToken) {
      return;
    }

    let ignore = false;
    loadOrganizationWorkspace(organizationToken)
      .then(() => {
        if (!ignore) {
          setNotice("");
        }
      })
      .catch(() => {
        if (!ignore) {
          clearOrganizationAuthStorage();
          setOrganizationToken("");
          setOrganizationTokenExpiry("");
          signOutOrganization(false);
        }
      });

    return () => {
      ignore = true;
    };
  }, [organizationToken]);

  useEffect(() => {
    let ignore = false;

    async function buildQRCodes() {
      if (!signedIn || typeof window === "undefined") {
        return {};
      }

      const activeItems = claims.filter((claim) => claim.status === "active");
      if (activeItems.length === 0) {
        return {};
      }

      const entries = await Promise.all(
        activeItems.map(async (claim) => {
          const url = new URL(`/verify/${claim.id}`, window.location.origin)
            .toString();
          const image = await QRCode.toDataURL(url, {
            errorCorrectionLevel: "M",
            margin: 1,
            width: 160,
            color: {
              dark: "#0f172a",
              light: "#ffffff",
            },
          });

          return [claim.id, image] as const;
        }),
      );

      return Object.fromEntries(entries);
    }

    buildQRCodes()
      .then((images) => {
        if (!ignore) {
          setClaimQRCodes(images);
        }
      })
      .catch(() => {
        if (!ignore) {
          setClaimQRCodes({});
        }
      });

    return () => {
      ignore = true;
    };
  }, [claims, signedIn]);

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
      setSecurityPIN("");
      setActivityItems(await loadActivityItems(token));
      setNotice(
        result.security_pin_set
          ? "Security PIN set. Future claim approvals will require it."
          : "Security PIN was not updated.",
      );
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleResetPIN(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      setError("Please sign in before resetting your Security PIN.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const result = await apiRequest<PinResponse>(
        "/account/security-pin/reset",
        {
          method: "POST",
          token,
          body: JSON.stringify({
            password: resetPINForm.password,
            security_pin: resetPINForm.securityPIN,
          }),
        },
      );
      setResetPINForm(emptyResetPINForm);
      setActivityItems(await loadActivityItems(token));
      setNotice(
        result.security_pin_set
          ? "Security PIN reset. Future claim approvals will require the new PIN."
          : "Security PIN was not reset.",
      );
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleEvidenceUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      setError("Please sign in before adding a record.");
      return;
    }

    if (!evidenceForm.file) {
      setError("Choose a file for this record first.");
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
      setActivityItems(await loadActivityItems(token));
      setEvidenceForm(emptyEvidenceForm);
      setNotice("Record added.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleRequestEvidenceReview(evidenceID: string) {
    if (!token) {
      setError("Please sign in before requesting record review.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const item = await apiRequest<EvidenceItem>(
        `/evidence-items/${evidenceID}/request-review`,
        {
          method: "POST",
          token,
        },
      );
      setEvidenceItems((items) =>
        items.map((record) => (record.id === item.id ? item : record)),
      );
      setActivityItems(await loadActivityItems(token));
      setNotice("Record review requested.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreateOrganizationRequest(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault();

    const userEmail = organizationRequestForm.userEmail.trim();
    const purpose = organizationRequestForm.purpose.trim();
    const supportedProofKeys = new Set(proofOptions.map((proof) => proof.key));
    const requestedTruths = organizationRequestForm.requestedTruths.filter(
      (truth) => supportedProofKeys.has(truth),
    );

    if (!organizationToken) {
      setError("Sign in to your organization before sending a request.");
      return;
    }

    if (!userEmail) {
      setError("Enter the user's email address.");
      return;
    }

    if (!purpose) {
      setError("Enter a clear request purpose.");
      return;
    }

    if (requestedTruths.length === 0) {
      setError("Choose at least one proof for the request.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const request = await apiRequest<ClaimRequest>(
        "/organization/claim-requests",
        {
          method: "POST",
          token: organizationToken,
          body: JSON.stringify({
            user_email: userEmail,
            purpose,
            requested_truths: requestedTruths,
            duration_days: Number(organizationRequestForm.durationDays),
          }),
        },
      );
      setCreatedOrganizationRequest(request);
      setOrganizationProfile((profile) => profile ?? request.organization);
      setOrganizationClaimRequests((requests) => [
        request,
        ...requests.filter((item) => item.id !== request.id),
      ]);
      setOrganizationRequestForm(emptyOrganizationRequestForm);
      setNotice(
        "Organization request sent. The user must approve it with their Security PIN before any proof is released.",
      );
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleRegisterOrganization(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    clearMessages();

    try {
      await apiRequest<Organization>("/organizations", {
        method: "POST",
        body: JSON.stringify({
          name: organizationRegisterForm.name,
          email: organizationRegisterForm.email,
          password: organizationRegisterForm.password,
          organization_type: organizationRegisterForm.organizationType,
        }),
      });

      const login = await organizationLoginWith(
        organizationRegisterForm.email,
        organizationRegisterForm.password,
      );
      setOrganizationRegisterForm(emptyOrganizationRegisterForm);
      await loadOrganizationWorkspace(login.access_token);
      setNotice("Organization account created and signed in.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleLoginOrganization(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    clearMessages();

    try {
      const login = await organizationLoginWith(
        organizationLoginForm.email,
        organizationLoginForm.password,
      );
      setOrganizationLoginForm(emptyOrganizationLoginForm);
      await loadOrganizationWorkspace(login.access_token);
      setNotice("Organization signed in.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function loadOrganizationWorkspace(accessToken: string) {
    const [
      profile,
      requestsResponse,
      claimsResponse,
      webhookEndpoint,
      webhookDeliveriesResponse,
      activityResponse,
      definitions,
    ] = await Promise.all([
      apiRequest<Organization>("/organization/me", {
        method: "GET",
        token: accessToken,
      }),
      apiRequest<ClaimRequestListResponse>("/organization/claim-requests", {
        method: "GET",
        token: accessToken,
      }),
      apiRequest<ClaimListResponse>("/organization/claims", {
        method: "GET",
        token: accessToken,
      }),
      loadOrganizationWebhookEndpoint(accessToken),
      loadOrganizationWebhookDeliveries(accessToken),
      loadOrganizationActivityItems(accessToken),
      loadTruthDefinitions(accessToken),
    ]);

    setOrganizationProfile(profile);
    setOrganizationClaimRequests(requestsResponse.items);
    setOrganizationClaims(claimsResponse.items);
    setOrganizationWebhookEndpoint(webhookEndpoint);
    setOrganizationWebhookURL(webhookEndpoint?.url ?? "");
    setOrganizationWebhookDeliveries(webhookDeliveriesResponse.items);
    setOrganizationActivityItems(activityResponse.items);
    setTruthDefinitions(definitions);
  }

  async function handleConfigureOrganizationWebhookEndpoint(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault();

    const url = organizationWebhookURL.trim();
    if (!organizationToken) {
      setError("Sign in to your organization before saving a webhook endpoint.");
      return;
    }
    if (!url) {
      setError("Enter a webhook endpoint URL.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const endpoint = await apiRequest<WebhookEndpoint>(
        "/organization/webhook-endpoint",
        {
          method: "POST",
          token: organizationToken,
          body: JSON.stringify({ url }),
        },
      );
      const deliveriesResponse =
        await loadOrganizationWebhookDeliveries(organizationToken);
      const activityResponse =
        await loadOrganizationActivityItems(organizationToken);
      setOrganizationWebhookEndpoint(endpoint);
      setOrganizationWebhookURL(endpoint.url);
      setOrganizationWebhookDeliveries(deliveriesResponse.items);
      setOrganizationActivityItems(activityResponse.items);
      setNotice("Webhook endpoint saved.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleApproveClaimRequest(
    event: FormEvent<HTMLFormElement>,
    requestID: string,
  ) {
    event.preventDefault();
    if (!token) {
      setError("Please sign in before approving a request.");
      return;
    }

    const securityPIN = approvalPINs[requestID] ?? "";
    setIsSubmitting(true);
    clearMessages();

    try {
      const result = await apiRequest<ApprovalResponse>(
        `/claim-requests/${requestID}/approve`,
        {
          method: "POST",
          token,
          body: JSON.stringify({ security_pin: securityPIN }),
        },
      );
      setClaimRequests((requests) =>
        requests.map((request) =>
          request.id === requestID ? result.claim_request : request,
        ),
      );
      setClaims(await loadClaims(token));
      setActivityItems(await loadActivityItems(token));
      setApprovalPINs((pins) => ({ ...pins, [requestID]: "" }));
      setNotice("Request approved. A time-bound proof is now active.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleDenyClaimRequest(requestID: string) {
    if (!token) {
      setError("Please sign in before denying a request.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const deniedRequest = await apiRequest<ClaimRequest>(
        `/claim-requests/${requestID}/deny`,
        {
          method: "POST",
          token,
        },
      );
      setClaimRequests((requests) =>
        requests.map((request) =>
          request.id === requestID ? deniedRequest : request,
        ),
      );
      setActivityItems(await loadActivityItems(token));
      setApprovalPINs((pins) => ({ ...pins, [requestID]: "" }));
      setNotice("Request denied. No proof was released.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleRevokeClaim(claimID: string) {
    if (!token) {
      setError("Please sign in before revoking a proof.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const revokedClaim = await apiRequest<Claim>(
        `/claims/${claimID}/revoke`,
        {
          method: "POST",
          token,
        },
      );
      setClaims((items) =>
        items.map((claim) => (claim.id === claimID ? revokedClaim : claim)),
      );
      setClaimExchangePINs((items) => {
        const next = { ...items };
        delete next[claimID];
        return next;
      });
      setActivityItems(await loadActivityItems(token));
      setNotice("Proof revoked. Its proof details are now hidden.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreateExchangePIN(claimID: string) {
    if (!token) {
      setError("Please sign in before creating an exchange PIN.");
      return;
    }

    setIsSubmitting(true);
    clearMessages();

    try {
      const exchangePIN = await apiRequest<ExchangePIN>(
        `/claims/${claimID}/exchange-pin`,
        {
          method: "POST",
          token,
        },
      );
      setClaimExchangePINs((items) => ({
        ...items,
        [claimID]: exchangePIN,
      }));
      setActivityItems(await loadActivityItems(token));
      setNotice("Temporary exchange PIN created.");
    } catch (err) {
      setError(readError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCopyClaimLink(claimID: string) {
    if (typeof window === "undefined") {
      return;
    }

    clearMessages();

    try {
      const url = new URL(`/verify/${claimID}`, window.location.origin)
        .toString();
      await window.navigator.clipboard.writeText(url);
      setCopiedClaimID(claimID);
      setNotice("Verification link copied.");
    } catch {
      setError("Unable to copy verification link.");
    }
  }

  async function handleCopyExchangePIN(claimID: string) {
    if (typeof window === "undefined") {
      return;
    }

    const exchangePIN = claimExchangePINs[claimID]?.exchange_pin;
    if (!exchangePIN) {
      setError("Create an exchange PIN first.");
      return;
    }

    clearMessages();

    try {
      await window.navigator.clipboard.writeText(exchangePIN);
      setCopiedExchangePINClaimID(claimID);
      setNotice("Exchange PIN copied.");
    } catch {
      setError("Unable to copy exchange PIN.");
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

  async function organizationLoginWith(email: string, password: string) {
    const login = await apiRequest<OrganizationLoginResponse>(
      "/organization/auth/login",
      {
        method: "POST",
        body: JSON.stringify({ email, password }),
      },
    );
    setOrganizationToken(login.access_token);
    setOrganizationTokenExpiry(login.expires_at);
    setOrganizationProfile(login.organization);
    window.sessionStorage.setItem(
      "kladd_organization_access_token",
      login.access_token,
    );
    window.sessionStorage.setItem(
      "kladd_organization_token_expiry",
      login.expires_at,
    );
    return login;
  }

  function signOut() {
    setToken("");
    setTokenExpiry("");
    setCurrentUser(null);
    setEvidenceItems([]);
    setClaimRequests([]);
    setClaims([]);
    setActivityItems([]);
    setApprovalPINs({});
    setClaimQRCodes({});
    setClaimExchangePINs({});
    setCopiedClaimID("");
    setCopiedExchangePINClaimID("");
    setEvidenceForm(emptyEvidenceForm);
    setResetPINForm(emptyResetPINForm);
    clearAuthStorage();
    setNotice("Signed out.");
    setError("");
  }

  function signOutOrganization(showNotice = true) {
    setOrganizationToken("");
    setOrganizationTokenExpiry("");
    setOrganizationProfile(null);
    setOrganizationClaimRequests([]);
    setOrganizationClaims([]);
    setOrganizationWebhookEndpoint(null);
    setOrganizationWebhookURL("");
    setOrganizationWebhookDeliveries([]);
    setOrganizationActivityItems([]);
    setCreatedOrganizationRequest(null);
    setOrganizationRequestForm(emptyOrganizationRequestForm);
    setOrganizationLoginForm(emptyOrganizationLoginForm);
    clearOrganizationAuthStorage();
    if (showNotice) {
      setNotice("Organization signed out.");
    }
    setError("");
  }

  function clearMessages() {
    setNotice("");
    setError("");
  }

  function handleWorkspaceNavigation(item: (typeof navItems)[number]) {
    setWorkspaceMode("personal");
    setError("");

    const targetID =
      item.requiresAuth && !signedIn ? "account-access" : item.targetID;

    if (item.requiresAuth && !signedIn) {
      setNotice(`Sign in to open ${item.label}.`);
    } else {
      setNotice("");
    }

    window.requestAnimationFrame(() => {
      document.getElementById(targetID)?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
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

            <nav
              className="flex flex-col gap-3 sm:items-end"
              aria-label="Workspace navigation"
            >
              <div className="grid w-full grid-cols-2 rounded-lg bg-slate-200/70 p-1 sm:w-[360px]">
                <button
                  type="button"
                  onClick={() => setWorkspaceMode("personal")}
                  className={workspaceButtonClass(workspaceMode === "personal")}
                >
                  Personal
                </button>
                <button
                  type="button"
                  onClick={() => setWorkspaceMode("organization")}
                  className={workspaceButtonClass(
                    workspaceMode === "organization",
                  )}
                >
                  Organization
                </button>
              </div>
              <div className="hidden flex-wrap gap-2 md:flex">
                {navItems.map((item) => (
                  <button
                    type="button"
                    key={item.label}
                    onClick={() => handleWorkspaceNavigation(item)}
                    className="rounded-md border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-800"
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            </nav>
          </div>
        </header>

        <section
          className={`grid flex-1 gap-5 py-5 ${
            workspaceMode === "organization"
              ? "lg:grid-cols-[minmax(0,1fr)_340px]"
              : "lg:grid-cols-[minmax(0,1fr)_390px]"
          }`}
        >
          <div className="space-y-5">
            <section
              id="workspace-home"
              className="scroll-mt-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            >
              <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div>
                  <p className="text-sm font-semibold text-emerald-700">
                    {workspaceMode === "personal"
                      ? "Personal workspace"
                      : "Organization workspace"}
                  </p>
                  <h2 className="mt-1 text-2xl font-semibold tracking-normal text-slate-950">
                    {workspaceMode === "personal"
                      ? "Control your proofs"
                      : "Request proofs without handling documents"}
                  </h2>
                  <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
                    {workspaceMode === "personal"
                      ? "Keep your records ready, review requests clearly, and approve only the proofs you want to release."
                      : "Send proof requests, wait for the user to approve, and review only the released verification result."}
                  </p>
                </div>

                {workspaceMode === "personal" && signedIn ? (
                  <button
                    type="button"
                    onClick={signOut}
                    className="h-10 rounded-md border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 shadow-sm transition hover:border-slate-400 hover:bg-slate-50"
                  >
                    Sign out
                  </button>
                ) : null}
                {workspaceMode === "organization" && organizationSignedIn ? (
                  <button
                    type="button"
                    onClick={() => signOutOrganization()}
                    className="h-10 rounded-md border border-emerald-200 bg-white px-4 text-sm font-semibold text-emerald-800 shadow-sm transition hover:border-emerald-300 hover:bg-emerald-50"
                  >
                    Sign out organization
                  </button>
                ) : null}
              </div>

              {workspaceMode === "personal" ? (
                <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  {statusCards.map((card) => (
                    <article
                      key={card.label}
                      className="min-h-28 rounded-lg border border-slate-200 bg-[#f9fbfd] p-4"
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
              ) : organizationSignedIn ? (
                <div className="mt-5 grid gap-3 sm:grid-cols-3">
                  <OrganizationMetric
                    label="Waiting"
                    value={String(organizationPendingRequests.length)}
                  />
                  <OrganizationMetric
                    label="Active"
                    value={String(organizationActiveClaims.length)}
                  />
                  <OrganizationMetric
                    label="Closed"
                    value={String(organizationClosedClaims.length)}
                  />
                </div>
              ) : (
                <div className="mt-5 rounded-lg border border-dashed border-emerald-200 bg-[#f7fbf8] p-4">
                  <p className="text-sm font-semibold text-slate-950">
                    Organization not signed in
                  </p>
                  <p className="mt-2 text-sm leading-6 text-slate-600">
                    Use the Organization sign in panel to load requester tools.
                  </p>
                </div>
              )}
            </section>

            {workspaceMode === "personal" && signedIn && currentUser ? (
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
                      value={formatCategory(currentUser.account_type)}
                    />
                    <ProfileField
                      label="Signed in until"
                      value={formatDateTime(tokenExpiry)}
                    />
                  </dl>
                </section>

                <section
                  id="active-proofs"
                  className="scroll-mt-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
                >
                  <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        Proofs
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        Active proofs
                      </h2>
                      <p className="mt-2 text-sm leading-6 text-slate-600">
                        These proofs are currently active. Expired or revoked
                        proofs keep their history but hide proof details.
                      </p>
                    </div>
                    <span className="w-fit rounded-md bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-800">
                      {activeClaims.length} active
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3">
                    {claims.length > 0 ? (
                      claims.map((claim) => (
                        <ClaimCard
                          key={claim.id}
                          claim={claim}
                          isSubmitting={isSubmitting}
                          copied={copiedClaimID === claim.id}
                          qrCodeSrc={claimQRCodes[claim.id] ?? ""}
                          exchangePIN={claimExchangePINs[claim.id] ?? null}
                          exchangePINCopied={
                            copiedExchangePINClaimID === claim.id
                          }
                          onCopyLink={() => handleCopyClaimLink(claim.id)}
                          onCreateExchangePIN={() =>
                            handleCreateExchangePIN(claim.id)
                          }
                          onCopyExchangePIN={() =>
                            handleCopyExchangePIN(claim.id)
                          }
                          onRevoke={() => handleRevokeClaim(claim.id)}
                        />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500">
                        No active proofs yet.
                      </div>
                    )}
                  </div>
                </section>

                <section
                  id="proof-requests"
                  className="scroll-mt-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
                >
                  <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        Requests
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        Pending proof requests
                      </h2>
                      <p className="mt-2 text-sm leading-6 text-slate-600">
                        Review who is asking, what they need, and why before
                        anything is approved.
                      </p>
                    </div>
                    <span className="w-fit rounded-md bg-amber-50 px-3 py-2 text-sm font-semibold text-amber-800">
                      {pendingClaimRequests.length} pending
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3">
                    {pendingClaimRequests.length > 0 ? (
                      pendingClaimRequests.map((request) => (
                        <ClaimRequestCard
                          key={request.id}
                          request={request}
                          approvalPIN={approvalPINs[request.id] ?? ""}
                          isSubmitting={isSubmitting}
                          onPINChange={(value) =>
                            setApprovalPINs((pins) => ({
                              ...pins,
                              [request.id]: value,
                            }))
                          }
                          onApprove={(event) =>
                            handleApproveClaimRequest(event, request.id)
                          }
                          onDeny={() => handleDenyClaimRequest(request.id)}
                        />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500">
                        No pending requests.
                      </div>
                    )}
                  </div>
                </section>

                <section
                  id="my-records"
                  className="scroll-mt-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
                >
                  <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        Access History
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        Recent activity
                      </h2>
                      <p className="mt-2 text-sm leading-6 text-slate-600">
                        Track important account and proof actions.
                      </p>
                    </div>
                    <span className="w-fit rounded-md bg-slate-100 px-3 py-2 text-sm font-semibold text-slate-700">
                      {activityItems.length} events
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3">
                    {activityItems.length > 0 ? (
                      activityItems.map((item) => (
                        <ActivityCard key={item.id} item={item} />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500">
                        No activity yet.
                      </div>
                    )}
                  </div>
                </section>

                <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                  <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-500">
                        My Records
                      </p>
                      <h2 className="mt-1 text-xl font-semibold tracking-normal">
                        Your saved records
                      </h2>
                      <p className="mt-2 text-sm leading-6 text-slate-600">
                        Store documents here so Kladd can prepare approved
                        proofs later. Requesters do not receive raw files by
                        default.
                      </p>
                    </div>
                    <span className="w-fit rounded-md bg-indigo-50 px-3 py-2 text-sm font-semibold text-indigo-800">
                      {evidenceItems.length} records
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    {evidenceItems.length > 0 ? (
                      evidenceItems.map((item) => (
                        <EvidenceCard
                          key={item.id}
                          item={item}
                          isSubmitting={isSubmitting}
                          onRequestReview={() =>
                            handleRequestEvidenceReview(item.id)
                          }
                        />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500 md:col-span-2">
                        No records yet. Add your first record from the panel on
                        the right.
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
                        Proof readiness
                      </h2>
                      <p className="mt-2 text-sm leading-6 text-slate-600">
                        See which proofs already have matching records and which
                        ones still need a record before they can be approved.
                      </p>
                    </div>
                    <span className="w-fit rounded-md bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-800">
                      {readyProofCount} ready
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    {proofReadiness.length > 0 ? (
                      proofReadiness.map((proof) => (
                        <ProofReadinessCard key={proof.key} proof={proof} />
                      ))
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-300 bg-[#f9fbfd] p-5 text-sm font-medium text-slate-500 md:col-span-2">
                        Proof readiness will appear after the Truth Registry
                        loads.
                      </div>
                    )}
                  </div>
                </section>
              </>
            ) : null}

            {workspaceMode === "organization" ? (
              <section className="rounded-lg border border-emerald-100 bg-[#f7fbf8] p-5 shadow-sm">
                <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                  <div>
                    <p className="text-sm font-semibold text-emerald-700">
                      For requesters
                    </p>
                    <h2 className="mt-1 text-xl font-semibold tracking-normal">
                      Send a proof request
                    </h2>
                    <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
                      Organizations request a proof. The user reviews it and
                      approves with their own Security PIN before Kladd releases
                      any verification result.
                    </p>
                  </div>
                  {organizationProfile ? (
                    <span className="w-fit rounded-md bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-800">
                      {organizationProfile.name}
                    </span>
                  ) : null}
                </div>

                {organizationSignedIn ? (
                  <>
                <form
                  className="mt-5 grid gap-4 lg:grid-cols-2"
                  onSubmit={handleCreateOrganizationRequest}
                >
                  <div className="space-y-4">
                    <TextInput
                      label="User email"
                      type="email"
                      value={organizationRequestForm.userEmail}
                      onChange={(value) =>
                        setOrganizationRequestForm((form) => ({
                          ...form,
                          userEmail: value,
                        }))
                      }
                      required
                    />

                    <TextInput
                      label="Request purpose"
                      value={organizationRequestForm.purpose}
                      onChange={(value) =>
                        setOrganizationRequestForm((form) => ({
                          ...form,
                          purpose: value,
                        }))
                      }
                      required
                    />

                    <label className="block">
                      <span className="text-sm font-semibold text-slate-700">
                        Access duration
                      </span>
                      <select
                        value={organizationRequestForm.durationDays}
                        onChange={(event) =>
                          setOrganizationRequestForm((form) => ({
                            ...form,
                            durationDays: event.target.value,
                          }))
                        }
                        className="mt-2 h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-950 outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100"
                      >
                        <option value="7">7 days</option>
                        <option value="30">30 days</option>
                        <option value="90">90 days</option>
                        <option value="180">180 days</option>
                      </select>
                    </label>
                  </div>

                  <div className="space-y-4">
                    <fieldset className="space-y-2">
                      <legend className="text-sm font-semibold text-slate-700">
                        Proofs requested
                      </legend>
                      <div className="grid gap-2 sm:grid-cols-2">
                        {proofOptions.map((proof) => (
                          <label
                            key={proof.key}
                            className="flex items-center gap-2 rounded-md border border-emerald-100 bg-white px-3 py-2 text-sm font-medium text-slate-700"
                          >
                            <input
                              type="checkbox"
                              checked={organizationRequestForm.requestedTruths.includes(
                                proof.key,
                              )}
                              onChange={(event) =>
                                setOrganizationRequestForm((form) => ({
                                  ...form,
                                  requestedTruths: event.target.checked
                                    ? [...form.requestedTruths, proof.key]
                                    : form.requestedTruths.filter(
                                        (truth) => truth !== proof.key,
                                      ),
                                }))
                              }
                              className="h-4 w-4 rounded border-slate-300 text-emerald-700 focus:ring-emerald-500"
                            />
                            <span>
                              <span className="block">{proof.label}</span>
                              <span className="block text-xs font-medium text-slate-500">
                                {proof.category}
                              </span>
                            </span>
                          </label>
                        ))}
                      </div>
                    </fieldset>

                    <div className="rounded-lg border border-emerald-100 bg-white p-4">
                      <p className="text-sm font-semibold text-slate-950">
                        User approval required
                      </p>
                      <p className="mt-2 text-sm leading-6 text-slate-600">
                        Your organization can send the request and track its
                        status, but only the user can approve or deny it.
                      </p>
                    </div>

                    <SubmitButton disabled={isSubmitting}>
                      Send request
                    </SubmitButton>
                  </div>
                </form>

                {createdOrganizationRequest ? (
                  <div className="mt-5 rounded-lg border border-emerald-200 bg-white p-4">
                    <p className="text-sm font-semibold text-slate-950">
                      Request sent
                    </p>
                    <p className="mt-1 text-sm leading-6 text-slate-600">
                      Waiting for the user to review and approve this proof
                      request.
                    </p>
                  </div>
                ) : null}

                  <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
                    <div className="space-y-4">
                      <div className="rounded-lg border border-emerald-200 bg-white p-4">
                        <p className="text-sm font-semibold text-slate-950">
                          {organizationProfile?.name ?? "Organization"}
                        </p>
                        <p className="mt-1 text-sm text-slate-600">
                          {formatCategory(
                            organizationProfile?.organization_type ?? "requester",
                          )}
                        </p>
                        <span className="mt-3 inline-flex w-fit rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-semibold capitalize text-emerald-800">
                          {formatRecordStatus(
                            organizationProfile?.verification_status ?? "loaded",
                          )}
                        </span>
                      </div>

                      <div className="grid gap-4 lg:grid-cols-3">
                        <OrganizationRequestList
                          title="Waiting for users"
                          emptyText="No user approvals are waiting."
                          requests={organizationPendingRequests}
                        />

                        <OrganizationClaimList
                          title="Active proofs"
                          emptyText="No active proofs yet."
                          claims={organizationActiveClaims}
                        />

                        <OrganizationClaimList
                          title="Closed proofs"
                          emptyText="No expired or revoked proofs yet."
                          claims={organizationClosedClaims}
                        />
                      </div>
                    </div>

                    <div className="space-y-4">
                      <section className="rounded-lg border border-emerald-100 bg-white p-4">
                        <div>
                          <p className="text-sm font-semibold text-slate-950">
                            Webhook endpoint
                          </p>
                          <p className="mt-1 text-sm leading-6 text-slate-600">
                            Receive proof status updates from Kladd.
                          </p>
                        </div>

                        <form
                          className="mt-4 space-y-3"
                          onSubmit={handleConfigureOrganizationWebhookEndpoint}
                        >
                          <TextInput
                            label="Endpoint URL"
                            type="url"
                            value={organizationWebhookURL}
                            onChange={setOrganizationWebhookURL}
                            required
                          />
                          <SubmitButton disabled={isSubmitting}>
                            Save endpoint
                          </SubmitButton>
                        </form>

                        {organizationWebhookEndpoint ? (
                          <div className="mt-4 rounded-lg border border-emerald-100 bg-[#f7fbf8] p-3">
                            <p className="break-all text-sm font-semibold text-slate-950">
                              {organizationWebhookEndpoint.url}
                            </p>
                            <p className="mt-1 text-xs font-medium text-slate-500">
                              Updated{" "}
                              {formatDateTime(
                                organizationWebhookEndpoint.updated_at,
                              )}
                            </p>
                          </div>
                        ) : (
                          <p className="mt-4 rounded-lg border border-dashed border-emerald-200 bg-[#f7fbf8] p-3 text-sm font-medium text-slate-500">
                            No endpoint saved yet.
                          </p>
                        )}
                      </section>

                      <OrganizationWebhookDeliveryList
                        deliveries={organizationWebhookDeliveries}
                      />

                      <OrganizationActivityList
                        items={organizationActivityItems}
                      />
                    </div>
                  </div>
                  </>
                ) : (
                  <div className="mt-5 rounded-lg border border-dashed border-emerald-200 bg-white p-5">
                    <p className="text-sm font-semibold text-slate-950">
                      Organization access required
                    </p>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      Use the Organization sign in panel to load your
                      organization workspace before sending requests or viewing
                      requester activity.
                    </p>
                  </div>
                )}
              </section>
            ) : null}
          </div>

          <aside className="space-y-5">
            {workspaceMode === "personal" && !signedIn ? (
            <section
              id="account-access"
              className="scroll-mt-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            >
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
            ) : null}

            {workspaceMode === "organization" ? (
              <section className="rounded-lg border border-emerald-100 bg-white p-5 shadow-sm">
                <div className="grid grid-cols-2 rounded-lg bg-emerald-50 p-1">
                  <button
                    type="button"
                    onClick={() => setOrganizationMode("register")}
                    className={modeButtonClass(organizationMode === "register")}
                  >
                    Register
                  </button>
                  <button
                    type="button"
                    onClick={() => setOrganizationMode("login")}
                    className={modeButtonClass(organizationMode === "login")}
                  >
                    Login
                  </button>
                </div>

                {organizationMode === "register" ? (
                  <form
                    className="mt-5 space-y-4"
                    onSubmit={handleRegisterOrganization}
                  >
                    <TextInput
                      label="Organization name"
                      value={organizationRegisterForm.name}
                      onChange={(value) =>
                        setOrganizationRegisterForm((form) => ({
                          ...form,
                          name: value,
                        }))
                      }
                      required
                    />
                    <TextInput
                      label="Work email"
                      type="email"
                      value={organizationRegisterForm.email}
                      onChange={(value) =>
                        setOrganizationRegisterForm((form) => ({
                          ...form,
                          email: value,
                        }))
                      }
                      required
                    />
                    <TextInput
                      label="Password"
                      type="password"
                      value={organizationRegisterForm.password}
                      onChange={(value) =>
                        setOrganizationRegisterForm((form) => ({
                          ...form,
                          password: value,
                        }))
                      }
                      minLength={8}
                      required
                    />
                    <label className="block">
                      <span className="text-sm font-semibold text-slate-700">
                        Organization type
                      </span>
                      <select
                        value={organizationRegisterForm.organizationType}
                        onChange={(event) =>
                          setOrganizationRegisterForm((form) => ({
                            ...form,
                            organizationType: event.target.value,
                          }))
                        }
                        className="mt-2 h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-950 outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100"
                      >
                        <option value="organization">Organization</option>
                        <option value="bank">Bank</option>
                        <option value="employer">Employer</option>
                        <option value="school">School</option>
                        <option value="healthcare">Healthcare</option>
                        <option value="government">Government</option>
                      </select>
                    </label>
                    <SubmitButton disabled={isSubmitting}>
                      Create organization
                    </SubmitButton>
                  </form>
                ) : (
                  <form
                    className="mt-5 space-y-4"
                    onSubmit={handleLoginOrganization}
                  >
                    <TextInput
                      label="Work email"
                      type="email"
                      value={organizationLoginForm.email}
                      onChange={(value) =>
                        setOrganizationLoginForm((form) => ({
                          ...form,
                          email: value,
                        }))
                      }
                      required
                    />
                    <TextInput
                      label="Password"
                      type="password"
                      value={organizationLoginForm.password}
                      onChange={(value) =>
                        setOrganizationLoginForm((form) => ({
                          ...form,
                          password: value,
                        }))
                      }
                      required
                    />
                    <SubmitButton disabled={isSubmitting}>
                      Sign in organization
                    </SubmitButton>
                  </form>
                )}

                {organizationSignedIn ? (
                  <p className="mt-4 rounded-lg border border-emerald-100 bg-[#f7fbf8] p-3 text-sm font-medium text-emerald-800">
                    Signed in until {formatDateTime(organizationTokenExpiry)}.
                  </p>
                ) : null}
              </section>
            ) : null}

            {workspaceMode === "personal" && signedIn ? (
              <>
            <section
              id="security"
              className="scroll-mt-4 rounded-lg border border-indigo-100 bg-[#f8f7ff] p-5 shadow-sm"
            >
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
                  required
                />
                <SubmitButton disabled={isSubmitting}>
                  Set Security PIN
                </SubmitButton>
              </form>

              <form
                className="mt-5 space-y-4 border-t border-indigo-100 pt-5"
                onSubmit={handleResetPIN}
              >
                <TextInput
                  label="Account password"
                  type="password"
                  value={resetPINForm.password}
                  onChange={(value) =>
                    setResetPINForm((form) => ({
                      ...form,
                      password: value,
                    }))
                  }
                  required
                />
                <TextInput
                  label="New Security PIN"
                  type="password"
                  inputMode="numeric"
                  value={resetPINForm.securityPIN}
                  onChange={(value) =>
                    setResetPINForm((form) => ({
                      ...form,
                      securityPIN: value,
                    }))
                  }
                  minLength={4}
                  maxLength={6}
                  required
                />
                <SubmitButton disabled={isSubmitting}>
                  Reset Security PIN
                </SubmitButton>
              </form>
            </section>

            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div>
                <p className="text-sm font-semibold text-slate-500">
                  My Records
                </p>
                <h2 className="mt-1 text-lg font-semibold tracking-normal">
                  Add a record
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
                    required
                    className="mt-2 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-950 outline-none transition file:mr-3 file:rounded-md file:border-0 file:bg-indigo-50 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-indigo-800 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
                  />
                </label>

                <SubmitButton disabled={isSubmitting}>
                  Add record
                </SubmitButton>
              </form>
            </section>
              </>
            ) : null}

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

function EvidenceCard({
  item,
  isSubmitting,
  onRequestReview,
}: {
  item: EvidenceItem;
  isSubmitting: boolean;
  onRequestReview: () => void;
}) {
  const canRequestReview = item.status === "uploaded";

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
          {formatRecordStatus(item.status)}
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

      {canRequestReview ? (
        <button
          type="button"
          onClick={onRequestReview}
          disabled={isSubmitting}
          className="mt-4 h-10 w-full rounded-md border border-indigo-200 bg-white px-4 text-sm font-semibold text-indigo-700 shadow-sm transition hover:border-indigo-300 hover:bg-indigo-50 disabled:cursor-not-allowed disabled:border-slate-200 disabled:bg-slate-100 disabled:text-slate-500"
        >
          Request review
        </button>
      ) : null}
    </article>
  );
}

function ClaimRequestCard({
  request,
  approvalPIN,
  isSubmitting,
  onPINChange,
  onApprove,
  onDeny,
}: {
  request: ClaimRequest;
  approvalPIN: string;
  isSubmitting: boolean;
  onPINChange: (value: string) => void;
  onApprove: (event: FormEvent<HTMLFormElement>) => void;
  onDeny: () => void;
}) {
  const canApprove = request.status === "pending_approval";

  return (
    <article className="rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <p className="text-sm font-semibold text-slate-950">
            {request.organization.name}
          </p>
          <p className="mt-1 text-sm text-slate-500">{request.purpose}</p>
        </div>
        <span className="w-fit rounded-md bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-800">
          {formatRequestStatus(request.status)}
        </span>
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        {request.requested_truths.map((truth) => (
          <span
            key={truth}
            className="rounded-md border border-slate-200 bg-white px-2.5 py-1 text-xs font-semibold text-slate-700"
          >
            {formatProofName(truth)}
          </span>
        ))}
      </div>

      <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-slate-500">Requested</dt>
          <dd className="mt-1 font-medium text-slate-800">
            {formatDateTime(request.created_at)}
          </dd>
        </div>
        <div>
          <dt className="text-slate-500">Valid until</dt>
          <dd className="mt-1 font-medium text-slate-800">
            {formatDateTime(request.expires_at)}
          </dd>
        </div>
      </dl>

      {canApprove ? (
        <form
          className="mt-4 grid gap-3 border-t border-slate-200 pt-4 sm:grid-cols-[minmax(0,1fr)_220px]"
          onSubmit={onApprove}
        >
          <TextInput
            label="Security PIN"
            type="password"
            inputMode="numeric"
            value={approvalPIN}
            onChange={onPINChange}
            minLength={4}
            maxLength={6}
            required
          />
          <div className="grid gap-2 sm:grid-cols-2 sm:items-end">
            <SubmitButton disabled={isSubmitting}>Approve</SubmitButton>
            <button
              type="button"
              onClick={onDeny}
              disabled={isSubmitting}
              className="h-11 w-full rounded-md border border-red-200 bg-white px-4 text-sm font-semibold text-red-700 shadow-sm transition hover:border-red-300 hover:bg-red-50 disabled:cursor-not-allowed disabled:border-slate-200 disabled:bg-slate-100 disabled:text-slate-500"
            >
              Deny
            </button>
          </div>
        </form>
      ) : null}
    </article>
  );
}

function ClaimCard({
  claim,
  isSubmitting,
  copied,
  qrCodeSrc,
  exchangePIN,
  exchangePINCopied,
  onCopyLink,
  onCreateExchangePIN,
  onCopyExchangePIN,
  onRevoke,
}: {
  claim: Claim;
  isSubmitting: boolean;
  copied: boolean;
  qrCodeSrc: string;
  exchangePIN: ExchangePIN | null;
  exchangePINCopied: boolean;
  onCopyLink: () => void;
  onCreateExchangePIN: () => void;
  onCopyExchangePIN: () => void;
  onRevoke: () => void;
}) {
  const canRevoke = claim.status === "active";

  return (
    <article className="rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <p className="text-sm font-semibold text-slate-950">
            {claim.organization.name}
          </p>
          <p className="mt-1 text-sm text-slate-500">{claim.purpose}</p>
        </div>
        <span className={claimStatusClass(claim.status)}>
          {formatClaimStatus(claim.status)}
        </span>
      </div>

      {claim.details_visible ? (
        <div className="mt-4 flex flex-wrap gap-2">
          {(claim.approved_truths ?? []).map((truth) => (
            <span
              key={truth}
              className="rounded-md border border-slate-200 bg-white px-2.5 py-1 text-xs font-semibold text-slate-700"
            >
              {formatProofName(truth)}
            </span>
          ))}
        </div>
      ) : (
        <p className="mt-4 rounded-md bg-white px-3 py-2 text-sm font-medium text-slate-600">
          Proof details are hidden for this {formatClaimStatus(claim.status).toLowerCase()} claim.
        </p>
      )}

      <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-slate-500">Issued</dt>
          <dd className="mt-1 font-medium text-slate-800">
            {formatDateTime(claim.issued_at)}
          </dd>
        </div>
        <div>
          <dt className="text-slate-500">Expires</dt>
          <dd className="mt-1 font-medium text-slate-800">
            {formatDateTime(claim.expires_at)}
          </dd>
        </div>
      </dl>

      {canRevoke && qrCodeSrc ? (
        <div className="mt-4 flex flex-col gap-3 rounded-lg border border-slate-200 bg-white p-3 sm:flex-row sm:items-center">
          <Image
            src={qrCodeSrc}
            alt="Verification QR code"
            width={112}
            height={112}
            unoptimized
            className="h-28 w-28 rounded-md border border-slate-200 bg-white p-1"
          />
          <div>
            <p className="text-sm font-semibold text-slate-950">
              Scan to verify
            </p>
            <p className="mt-1 text-sm leading-6 text-slate-600">
              Anyone with this QR code can open the current verification page
              for this proof.
            </p>
          </div>
        </div>
      ) : null}

      {canRevoke ? (
        <div className="mt-4 rounded-lg border border-slate-200 bg-white p-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-sm font-semibold text-slate-950">
                Temporary exchange PIN
              </p>
              <p className="mt-1 text-sm leading-6 text-slate-600">
                Share this only when someone needs to open the verification
                page without a QR code.
              </p>
            </div>
            <button
              type="button"
              onClick={onCreateExchangePIN}
              disabled={isSubmitting}
              className="h-10 rounded-md border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 shadow-sm transition hover:border-slate-400 hover:bg-slate-50 disabled:cursor-not-allowed disabled:border-slate-200 disabled:bg-slate-100 disabled:text-slate-500"
            >
              {exchangePIN ? "Refresh PIN" : "Create PIN"}
            </button>
          </div>

          {exchangePIN ? (
            <div className="mt-3 flex flex-col gap-2 rounded-md bg-[#f9fbfd] p-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="font-mono text-2xl font-semibold tracking-normal text-slate-950">
                  {exchangePIN.exchange_pin}
                </p>
                <p className="mt-1 text-xs font-medium text-slate-500">
                  Expires {formatDateTime(exchangePIN.expires_at)}
                </p>
              </div>
              <button
                type="button"
                onClick={onCopyExchangePIN}
                className="h-10 rounded-md border border-indigo-200 bg-white px-4 text-sm font-semibold text-indigo-700 shadow-sm transition hover:border-indigo-300 hover:bg-indigo-50"
              >
                {exchangePINCopied ? "Copied" : "Copy PIN"}
              </button>
            </div>
          ) : null}
        </div>
      ) : null}

      <div className="mt-4 flex flex-col gap-2 border-t border-slate-200 pt-4 sm:flex-row">
        <a
          href={`/verify/${claim.id}`}
          className="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 shadow-sm transition hover:border-slate-400 hover:bg-slate-50"
        >
          View verification
        </a>
        <button
          type="button"
          onClick={onCopyLink}
          className="h-10 rounded-md border border-indigo-200 bg-white px-4 text-sm font-semibold text-indigo-700 shadow-sm transition hover:border-indigo-300 hover:bg-indigo-50"
        >
          {copied ? "Copied" : "Copy link"}
        </button>
        {canRevoke ? (
          <button
            type="button"
            onClick={onRevoke}
            disabled={isSubmitting}
            className="h-10 rounded-md border border-red-200 bg-white px-4 text-sm font-semibold text-red-700 shadow-sm transition hover:border-red-300 hover:bg-red-50 disabled:cursor-not-allowed disabled:border-slate-200 disabled:bg-slate-100 disabled:text-slate-500"
          >
            Revoke proof
          </button>
        ) : null}
      </div>
    </article>
  );
}

function ActivityCard({ item }: { item: ActivityItem }) {
  return (
    <article className="rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-sm font-semibold text-slate-950">{item.title}</p>
          <p className="mt-1 text-sm leading-6 text-slate-600">
            {item.description}
          </p>
        </div>
        <span className="w-fit rounded-md bg-white px-2.5 py-1 text-xs font-semibold text-slate-600">
          {formatDateTime(item.created_at)}
        </span>
      </div>
    </article>
  );
}

function ProofReadinessCard({ proof }: { proof: ProofReadiness }) {
  return (
    <article className="min-h-40 rounded-lg border border-slate-200 bg-[#f9fbfd] p-4">
      <div className={`mb-4 h-1.5 w-16 rounded-full ${readinessAccentClass(proof.status)}`} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-slate-950">{proof.label}</p>
          <p className="mt-1 text-sm text-slate-500">{proof.category}</p>
        </div>
        <span className={proofReadinessStatusClass(proof.status)}>
          {formatProofReadiness(proof.status)}
        </span>
      </div>
      <p className="mt-2 text-sm leading-6 text-slate-600">
        {proofReadinessDescription(proof.status)}
      </p>
      <p className="mt-4 rounded-md bg-white px-3 py-2 text-sm font-medium text-slate-700">
        Needs: {proof.requiredEvidence.map(formatCategory).join(", ")}
      </p>
    </article>
  );
}

function OrganizationMetric({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-lg border border-emerald-100 bg-white p-3">
      <p className="text-xs font-semibold uppercase tracking-normal text-emerald-700">
        {label}
      </p>
      <p className="mt-2 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function OrganizationRequestList({
  title,
  emptyText,
  requests,
}: {
  title: string;
  emptyText: string;
  requests: ClaimRequest[];
}) {
  return (
    <div>
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <div className="mt-2 space-y-2">
        {requests.length > 0 ? (
          requests.map((request) => (
            <article
              key={request.id}
              className="rounded-lg border border-emerald-100 bg-white p-3"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-slate-950">
                    {request.purpose}
                  </p>
                  <p className="mt-1 text-xs font-medium text-slate-500">
                    Expires {formatDateTime(request.expires_at)}
                  </p>
                </div>
                <span className="w-fit rounded-md bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-800">
                  {formatRequestStatus(request.status)}
                </span>
              </div>
              <div className="mt-3 flex flex-wrap gap-1.5">
                {request.requested_truths.map((truth) => (
                  <span
                    key={truth}
                    className="rounded-md bg-[#f7fbf8] px-2 py-1 text-xs font-semibold text-slate-700"
                  >
                    {formatProofName(truth)}
                  </span>
                ))}
              </div>
            </article>
          ))
        ) : (
          <p className="rounded-lg border border-dashed border-emerald-200 bg-white p-3 text-sm font-medium text-slate-500">
            {emptyText}
          </p>
        )}
      </div>
    </div>
  );
}

function OrganizationClaimList({
  title,
  emptyText,
  claims,
}: {
  title: string;
  emptyText: string;
  claims: Claim[];
}) {
  return (
    <div>
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <div className="mt-2 space-y-2">
        {claims.length > 0 ? (
          claims.map((claim) => (
            <article
              key={claim.id}
              className="rounded-lg border border-emerald-100 bg-white p-3"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-slate-950">
                    {claim.purpose}
                  </p>
                  <p className="mt-1 text-xs font-medium text-slate-500">
                    Expires {formatDateTime(claim.expires_at)}
                  </p>
                </div>
                <span className={claimStatusClass(claim.status)}>
                  {formatClaimStatus(claim.status)}
                </span>
              </div>

              {claim.details_visible ? (
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {(claim.approved_truths ?? []).map((truth) => (
                    <span
                      key={truth}
                      className="rounded-md bg-[#f7fbf8] px-2 py-1 text-xs font-semibold text-slate-700"
                    >
                      {formatProofName(truth)}
                    </span>
                  ))}
                </div>
              ) : (
                <p className="mt-3 rounded-md bg-[#f7fbf8] px-3 py-2 text-xs font-semibold text-slate-600">
                  Proof details hidden
                </p>
              )}
            </article>
          ))
        ) : (
          <p className="rounded-lg border border-dashed border-emerald-200 bg-white p-3 text-sm font-medium text-slate-500">
            {emptyText}
          </p>
        )}
      </div>
    </div>
  );
}

function OrganizationWebhookDeliveryList({
  deliveries,
}: {
  deliveries: WebhookDelivery[];
}) {
  return (
    <div>
      <p className="text-sm font-semibold text-slate-950">Webhook history</p>
      <div className="mt-2 space-y-2">
        {deliveries.length > 0 ? (
          deliveries.map((delivery) => (
            <article
              key={delivery.id}
              className="rounded-lg border border-emerald-100 bg-white p-3"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-slate-950">
                    {formatWebhookEvent(delivery.event_type)}
                  </p>
                  <p className="mt-1 text-xs font-medium text-slate-500">
                    Created {formatDateTime(delivery.created_at)}
                  </p>
                </div>
                <span className={deliveryStatusClass(delivery.status)}>
                  {formatDeliveryStatus(delivery.status)}
                </span>
              </div>
              <dl className="mt-3 grid gap-2 text-xs sm:grid-cols-2">
                <div>
                  <dt className="font-semibold text-slate-500">Attempts</dt>
                  <dd className="mt-1 font-medium text-slate-800">
                    {delivery.attempts}
                  </dd>
                </div>
                <div>
                  <dt className="font-semibold text-slate-500">Delivered</dt>
                  <dd className="mt-1 font-medium text-slate-800">
                    {delivery.delivered_at
                      ? formatDateTime(delivery.delivered_at)
                      : "Not yet"}
                  </dd>
                </div>
                {delivery.next_attempt_at ? (
                  <div className="sm:col-span-2">
                    <dt className="font-semibold text-slate-500">
                      Next attempt
                    </dt>
                    <dd className="mt-1 font-medium text-slate-800">
                      {formatDateTime(delivery.next_attempt_at)}
                    </dd>
                  </div>
                ) : null}
              </dl>
            </article>
          ))
        ) : (
          <p className="rounded-lg border border-dashed border-emerald-200 bg-white p-3 text-sm font-medium text-slate-500">
            No webhook deliveries yet.
          </p>
        )}
      </div>
    </div>
  );
}

function OrganizationActivityList({ items }: { items: ActivityItem[] }) {
  return (
    <div>
      <p className="text-sm font-semibold text-slate-950">Access history</p>
      <div className="mt-2 space-y-2">
        {items.length > 0 ? (
          items.map((item) => (
            <article
              key={item.id}
              className="rounded-lg border border-emerald-100 bg-white p-3"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-slate-950">
                    {item.title}
                  </p>
                  <p className="mt-1 text-sm leading-6 text-slate-600">
                    {item.description}
                  </p>
                </div>
                <span className="w-fit rounded-md bg-[#f7fbf8] px-2.5 py-1 text-xs font-semibold text-slate-600">
                  {formatDateTime(item.created_at)}
                </span>
              </div>
            </article>
          ))
        ) : (
          <p className="rounded-lg border border-dashed border-emerald-200 bg-white p-3 text-sm font-medium text-slate-500">
            No organization activity yet.
          </p>
        )}
      </div>
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

function clearOrganizationAuthStorage() {
  window.sessionStorage.removeItem("kladd_organization_access_token");
  window.sessionStorage.removeItem("kladd_organization_token_expiry");
}

function modeButtonClass(active: boolean) {
  return `h-10 rounded-md text-sm font-semibold transition ${
    active
      ? "bg-white text-indigo-800 shadow-sm"
      : "text-slate-600 hover:text-slate-950"
  }`;
}

function workspaceButtonClass(active: boolean) {
  return `h-10 rounded-md text-sm font-semibold transition ${
    active
      ? "bg-white text-slate-950 shadow-sm"
      : "text-slate-600 hover:text-slate-950"
  }`;
}

async function apiRequest<T>(
  path: string,
  options: {
    method: "GET" | "POST";
    body?: string;
    token?: string;
    apiKey?: string;
  },
): Promise<T> {
  const headers = new Headers();
  headers.set("content-type", "application/json");
  if (options.token) {
    headers.set("authorization", `Bearer ${options.token}`);
  }
  if (options.apiKey) {
    headers.set("x-kladd-api-key", options.apiKey);
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

async function loadClaimRequests(accessToken: string) {
  const response = await apiRequest<ClaimRequestListResponse>(
    "/claim-requests",
    {
      method: "GET",
      token: accessToken,
    },
  );

  return response.items;
}

async function loadClaims(accessToken: string) {
  const response = await apiRequest<ClaimListResponse>("/claims", {
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

async function loadOrganizationWebhookEndpoint(accessToken: string) {
  try {
    return await apiRequest<WebhookEndpoint>("/organization/webhook-endpoint", {
      method: "GET",
      token: accessToken,
    });
  } catch (err) {
    if (readError(err) === "Webhook endpoint is not configured.") {
      return null;
    }

    throw err;
  }
}

async function loadOrganizationWebhookDeliveries(accessToken: string) {
  return apiRequest<WebhookDeliveryListResponse>(
    "/organization/webhook-deliveries",
    {
      method: "GET",
      token: accessToken,
    },
  );
}

async function loadOrganizationActivityItems(accessToken: string) {
  return apiRequest<ActivityListResponse>("/organization/audit-logs", {
    method: "GET",
    token: accessToken,
  });
}

async function loadActivityItems(accessToken: string) {
  const response = await apiRequest<ActivityListResponse>("/audit-logs", {
    method: "GET",
    token: accessToken,
  });

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

function formatRecordStatus(value: string) {
  if (value === "uploaded") {
    return "Added";
  }
  if (value === "pending_verification") {
    return "Review requested";
  }
  if (value === "verified") {
    return "Verified";
  }
  if (value === "rejected") {
    return "Needs review";
  }

  return formatCategory(value);
}

function formatRequestStatus(value: string) {
  if (value === "pending_approval") {
    return "Waiting for review";
  }
  if (value === "approved_with_security_pin") {
    return "Approved";
  }
  if (value === "approved") {
    return "Approved";
  }
  if (value === "denied") {
    return "Denied";
  }
  if (value === "expired") {
    return "Expired";
  }

  return formatCategory(value);
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

function formatProofReadiness(value: ProofReadiness["status"]) {
  if (value === "ready") {
    return "Ready";
  }
  if (value === "review") {
    return "Review needed";
  }

  return "Record needed";
}

function proofReadinessDescription(value: ProofReadiness["status"]) {
  if (value === "ready") {
    return "Matching records are verified, so this proof can be released after approval.";
  }
  if (value === "review") {
    return "Matching records are saved, but they still need verification before this proof is ready.";
  }

  return "Add the required record so Kladd can prepare this proof.";
}

function formatWebhookEvent(value: string) {
  if (value === "claim.approved") {
    return "Proof approved";
  }
  if (value === "claim.expired") {
    return "Proof expired";
  }
  if (value === "claim.revoked") {
    return "Proof revoked";
  }

  return formatCategory(value.replaceAll(".", "_"));
}

function formatDeliveryStatus(value: string) {
  if (value === "pending") {
    return "Pending";
  }
  if (value === "delivered") {
    return "Delivered";
  }
  if (value === "failed") {
    return "Failed";
  }

  return formatCategory(value);
}

function claimStatusClass(value: string) {
  const base = "w-fit rounded-md px-2.5 py-1 text-xs font-semibold";
  if (value === "active") {
    return `${base} bg-emerald-50 text-emerald-800`;
  }
  if (value === "expired") {
    return `${base} bg-slate-100 text-slate-700`;
  }
  if (value === "revoked") {
    return `${base} bg-red-50 text-red-800`;
  }

  return `${base} bg-amber-50 text-amber-800`;
}

function deliveryStatusClass(value: string) {
  const base = "w-fit rounded-md px-2.5 py-1 text-xs font-semibold";
  if (value === "delivered") {
    return `${base} bg-emerald-50 text-emerald-800`;
  }
  if (value === "failed") {
    return `${base} bg-red-50 text-red-800`;
  }

  return `${base} bg-amber-50 text-amber-800`;
}

function proofReadinessStatusClass(value: ProofReadiness["status"]) {
  const base = "w-fit rounded-md px-2.5 py-1 text-xs font-semibold";
  if (value === "ready") {
    return `${base} bg-emerald-50 text-emerald-800`;
  }
  if (value === "review") {
    return `${base} bg-amber-50 text-amber-800`;
  }

  return `${base} bg-slate-100 text-slate-700`;
}

function readinessAccentClass(value: ProofReadiness["status"]) {
  if (value === "ready") {
    return "bg-emerald-500";
  }
  if (value === "review") {
    return "bg-amber-500";
  }

  return "bg-indigo-600";
}
