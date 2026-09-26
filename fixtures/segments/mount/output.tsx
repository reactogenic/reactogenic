import { Card } from "./card";
import _Section_intro1 from "./+intro";
import _Card_pricing from "./+pricing";
import _UiPanel_faqList from "./+faq-list";
const _Section_intro = 1;
export const a = <section id="intro"><_Section_intro1 /></section>;
export const b = <Card id="pricing" className="band"><_Card_pricing /></Card>;
export const c = <Ui.Panel id="faq-list"><_UiPanel_faqList /></Ui.Panel>;
