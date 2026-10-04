// The shell's time zone is UTC (builder.md, *Shell code in phase 2*, S4): a
// date that is given is a compile-time value, the same on every machine —
// what Node renders under TZ=UTC (differential_test.go).
const day = new Date(2026, 9, 4, 9, 30);
const parsed = new Date("2026-10-04T23:30");
const moved = new Date(2026, 0, 31);
moved.setMonth(1);
moved.setHours(25, 61);
const century = new Date(99, 0);

export default function DatesPage() {
  return (
    <html lang="en">
      <body>
        <time dateTime={day.toISOString()}>{day.getTimezoneOffset()}</time>
        <p>
          {day.getFullYear()}-{day.getMonth()}-{day.getDate()} {day.getHours()}:{day.getMinutes()}:{day.getSeconds()}.
          {day.getMilliseconds()} day {day.getDay()}
        </p>
        <p>{parsed.toISOString()} {parsed.getDate()} {parsed.getHours()}</p>
        <p>{Date.parse("2026-10-04T00:00")} {Date.parse("2026-10-04")} {Date.parse("2026-10-04T10:00+02:00")} {Date.parse("2026-10-04T10:00:00.5Z")}</p>
        <p>{String(day)}</p>
        <p>{`${day}`} | {day.toDateString()} | {day.toTimeString()} | {day.toUTCString()}</p>
        <p>{moved.toISOString()} {century.toISOString()} {JSON.stringify({ day })}</p>
        <p>{new Date(String(day)).getTime() === day.getTime() ? "parses its own" : "does not parse its own"}</p>
        <p>{new Date(day.toUTCString()).getTime() === day.getTime() ? "parses toUTCString" : "does not"}</p>
        <p>{String(new Date(NaN))} {new Date(Date.UTC(-1, 0, 1)).toDateString()} {new Date(Date.UTC(12345, 0, 1)).toDateString()}</p>
      </body>
    </html>
  );
}
