export const a = (
  <div>
    {open ? <>{((_on) => _on === "a" ? <>{ok ? <b /> : null}</> : null)(getMode())}</> : null}
  </div>
);
