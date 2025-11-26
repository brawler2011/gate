"use server";

import { FrontendApi, Configuration, LoginFlow, RegistrationFlow, VerificationFlow, RecoveryFlow, SettingsFlow } from "@ory/client-fetch";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

const ory = new FrontendApi(
  new Configuration({
    basePath: process.env.ORY_SDK_URL,
  })
);

async function getCookieHeader(): Promise<string> {
  const cookieStore = await cookies();
  return cookieStore.getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join("; ");
}

export async function getLoginFlow(searchParams: Promise<{ flow?: string }> | { flow?: string }): Promise<LoginFlow | null> {
  const params = await searchParams;
  const flowId = params.flow;

  if (!flowId) {
    // No flow ID - redirect browser to Kratos to create flow (this sets CSRF cookie in browser)
    redirect("/api/.ory/self-service/login/browser");
  }

  try {
    const flow = await ory.getLoginFlow({
      id: flowId,
      cookie: await getCookieHeader(),
    });
    return flow;
  } catch (error: any) {
    // Flow expired or invalid - redirect to create new flow
    if (error?.response?.status === 410 || error?.response?.status === 404 || error?.response?.status === 403) {
      redirect("/api/.ory/self-service/login/browser");
    }
    console.error("Error fetching login flow:", error);
    return null;
  }
}

export async function getRegistrationFlow(searchParams: Promise<{ flow?: string }> | { flow?: string }): Promise<RegistrationFlow | null> {
  const params = await searchParams;
  const flowId = params.flow;

  if (!flowId) {
    redirect("/api/.ory/self-service/registration/browser");
  }

  try {
    const flow = await ory.getRegistrationFlow({
      id: flowId,
      cookie: await getCookieHeader(),
    });
    return flow;
  } catch (error: any) {
    if (error?.response?.status === 410 || error?.response?.status === 404 || error?.response?.status === 403) {
      redirect("/api/.ory/self-service/registration/browser");
    }
    console.error("Error fetching registration flow:", error);
    return null;
  }
}

export async function getVerificationFlow(searchParams: Promise<{ flow?: string }> | { flow?: string }): Promise<VerificationFlow | null> {
  const params = await searchParams;
  const flowId = params.flow;

  if (!flowId) {
    redirect("/api/.ory/self-service/verification/browser");
  }

  try {
    const flow = await ory.getVerificationFlow({
      id: flowId,
      cookie: await getCookieHeader(),
    });
    return flow;
  } catch (error: any) {
    if (error?.response?.status === 410 || error?.response?.status === 404 || error?.response?.status === 403) {
      redirect("/api/.ory/self-service/verification/browser");
    }
    console.error("Error fetching verification flow:", error);
    return null;
  }
}

export async function getRecoveryFlow(searchParams: Promise<{ flow?: string }> | { flow?: string }): Promise<RecoveryFlow | null> {
  const params = await searchParams;
  const flowId = params.flow;

  if (!flowId) {
    redirect("/api/.ory/self-service/recovery/browser");
  }

  try {
    const flow = await ory.getRecoveryFlow({
      id: flowId,
      cookie: await getCookieHeader(),
    });
    return flow;
  } catch (error: any) {
    if (error?.response?.status === 410 || error?.response?.status === 404 || error?.response?.status === 403) {
      redirect("/api/.ory/self-service/recovery/browser");
    }
    console.error("Error fetching recovery flow:", error);
    return null;
  }
}

export async function getSettingsFlow(searchParams: Promise<{ flow?: string }> | { flow?: string }): Promise<SettingsFlow | null> {
  const params = await searchParams;
  const flowId = params.flow;

  if (!flowId) {
    redirect("/api/.ory/self-service/settings/browser");
  }

  try {
    const flow = await ory.getSettingsFlow({
      id: flowId,
      cookie: await getCookieHeader(),
    });
    return flow;
  } catch (error: any) {
    if (error?.response?.status === 410 || error?.response?.status === 404 || error?.response?.status === 403) {
      redirect("/api/.ory/self-service/settings/browser");
    }
    console.error("Error fetching settings flow:", error);
    return null;
  }
}
