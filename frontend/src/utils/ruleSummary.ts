// Summaries of route and DNS rules for the list tables. `skip` holds the
// fields that describe what a rule does rather than what it matches.

// conditionKeys lists the match fields of a rule (of all sub-rules for a
// logical rule).
export const conditionKeys = (rule: any, skip: readonly string[]): string[] => {
  if (Array.isArray(rule?.rules)) {
    return [...new Set<string>(rule.rules.flatMap((sub: any) => conditionKeys(sub, skip)))]
  }
  return Object.keys(rule ?? {}).filter(key => !skip.includes(key) && key != 'type' && key != 'mode')
}

const conditionText = (value: unknown): string => {
  const text = Array.isArray(value) ? value.join(', ') : String(value)
  return text.length > 80 ? text.slice(0, 77) + '...' : text
}

// ruleSummary spells out the conditions, one per line, for a tooltip.
export const ruleSummary = (rule: any, skip: readonly string[]): string => {
  if (Array.isArray(rule?.rules)) {
    return rule.rules
      .map((sub: any, index: number) => `#${index + 1} ${ruleSummary(sub, skip).replace(/\n/g, '; ')}`)
      .join('\n')
  }
  return conditionKeys(rule, skip).map(key => `${key}: ${conditionText(rule[key])}`).join('\n')
}
