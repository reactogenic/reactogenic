import { value } from "./draft";
export function Field({ onChange }: { onChange: () => void }) {
  return <Input value={value} onChange={onChange} required />;
}
