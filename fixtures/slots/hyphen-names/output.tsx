export const a = (
  <Button $icon-start={{ title: "t" }} $Icon={{ "$sub-item": { title: "t" }, $Sub: {} }} />
);
export const b = (
  <Button
    $icon-start={{ children: ({ size }) => <>start {size}</> }}
    $Icon={{ "$sub-item": { children: ({ size }) => <>text {size}</> } }}
  />
);
