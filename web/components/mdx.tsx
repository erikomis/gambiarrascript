import defaultMdxComponents from "fumadocs-ui/mdx";
import { Callout } from "fumadocs-ui/components/callout";
import { Steps, Step } from "fumadocs-ui/components/steps";
import { TypeTable } from "fumadocs-ui/components/type-table";
import { Accordions, Accordion } from "fumadocs-ui/components/accordion";

export function getMDXComponents() {
  return {
    ...defaultMdxComponents,
    Callout,
    Steps,
    Step,
    TypeTable,
    Accordions,
    Accordion,
  };
}