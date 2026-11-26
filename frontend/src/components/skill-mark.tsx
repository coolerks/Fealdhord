import { CustomIcon, hexColorRegex } from '@/components/custom-icon'

type SkillAppearance = { icon?: string; color?: string }

export function SkillIcon({ icon, color }: SkillAppearance) {
  return <span className="inline-flex size-4 shrink-0 items-center justify-center"
    style={{ color: color && hexColorRegex.test(color) ? color : undefined }}>
    <CustomIcon value={icon} fallback="blocks" className="size-4" />
  </span>
}

export function SkillMarks({ skills }: { skills?: (SkillAppearance & { name: string })[] | null }) {
  if (!skills?.length) return null
  return <ul className="mb-1 flex flex-wrap gap-1.5" aria-label="已应用技能">
    {skills.map(skill => <li key={skill.name}
      className="inline-flex min-w-0 items-center gap-1 rounded-md border border-border/75 bg-background/60 px-1.5 py-0.5 text-xs">
      <SkillIcon {...skill} />
      <span className="truncate" title={skill.name}>{skill.name}</span>
    </li>)}
  </ul>
}
