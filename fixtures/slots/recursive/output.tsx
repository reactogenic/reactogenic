export const a = (
  <Dialog $Action={{ variant: "solid", $IconStart: { children: <Icon name="close" /> }, children: "Close" }} />
);
export const b = (
  <Dialog $Action={{ children: <Button $IconStart={{ children: "x" }} /> }} />
);
