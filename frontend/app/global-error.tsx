"use client";

type GlobalErrorProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function GlobalError({ error, reset }: GlobalErrorProps) {
  return (
    <html lang="ru">
      <body>
        <main
          style={{
            minHeight: "100vh",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            padding: "24px",
            background: "#1f1f1f",
            color: "#f1f1f1",
            fontFamily: "system-ui, sans-serif",
          }}
        >
          <section
            style={{
              maxWidth: "640px",
              width: "100%",
              border: "1px solid #3a3a3a",
              borderRadius: "12px",
              padding: "24px",
              background: "#262626",
            }}
          >
            <h1 style={{ margin: 0, marginBottom: "12px", fontSize: "1.5rem" }}>
              Произошла непредвиденная ошибка
            </h1>
            <p style={{ marginTop: 0, color: "#c6c6c6" }}>
              Попробуйте обновить страницу или вернуться на главную.
            </p>
            {error.digest ? (
              <p style={{ color: "#9c9c9c", fontSize: "0.9rem" }}>
                ID ошибки: {error.digest}
              </p>
            ) : null}
            <div style={{ display: "flex", gap: "12px", marginTop: "16px" }}>
              <button
                type="button"
                onClick={() => reset()}
                style={{
                  border: "none",
                  borderRadius: "8px",
                  padding: "10px 14px",
                  cursor: "pointer",
                  background: "#2f6df6",
                  color: "#ffffff",
                }}
              >
                Попробовать снова
              </button>
              <a href="/" style={{ color: "#d7d7d7", alignSelf: "center" }}>
                На главную
              </a>
            </div>
          </section>
        </main>
      </body>
    </html>
  );
}
