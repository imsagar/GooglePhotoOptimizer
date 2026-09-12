export function Login() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-bg-primary">
      <div className="bg-bg-card rounded-[10px] p-8 w-full max-w-md text-center border border-border">
        <h1 className="text-3xl font-bold text-accent mb-2">GPOptimizer</h1>
        <p className="text-text-secondary mb-1">Optimize your Google Photos videos</p>
        <p className="text-text-muted text-sm mb-8">Your videos never leave your machine</p>
        <a
          href="/api/auth/google"
          className="inline-flex items-center justify-center gap-2 bg-accent hover:bg-accent-hover text-white font-medium px-6 py-3 rounded-lg transition-colors w-full"
        >
          Sign in with Google
        </a>
      </div>
    </div>
  );
}
