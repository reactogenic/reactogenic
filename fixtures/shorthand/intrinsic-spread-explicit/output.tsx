export function F({ disabled, props }: any) {
  return <><button disabled={disabled} /><Input {...props} disabled={disabled} /><Button disabled={true} /></>;
}
