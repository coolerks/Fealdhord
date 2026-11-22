import { useId } from 'react'

type TimeSlot = 'morning' | 'noon' | 'afternoon' | 'evening' | 'night' | 'lateNight'

const greetingsBySlot: Record<TimeSlot, readonly string[]> = {
  morning: [
    '早上好，新的一天，有什么我可以帮你的？',
    '早上好，需要我为你做点什么吗？',
    '早上好，今天想先看看哪些内容？',
    '早安，今天有什么计划需要一起梳理吗？',
    '早上好，准备开始今天的工作了吗？',
  ],
  noon: [
    '中午好，有什么我可以帮你的吗？',
    '中午好，午休前想顺手处理点什么？',
    '中午好，今天上午有什么收获想聊聊吗？',
    '中午好，需要我帮你整理一下今天的资讯吗？',
    '中午好，接下来想查点什么资料？',
  ],
  afternoon: [
    '下午好，有什么我可以帮你的？',
    '下午好，手头有什么问题需要一起看看吗？',
    '下午好，需要我帮你检索或总结些什么吗？',
    '下午好，今天下午想先推进哪件事？',
    '下午好，需要我为你做点什么吗？',
  ],
  evening: [
    '傍晚好，今天辛苦了，还有什么需要帮忙的吗？',
    '傍晚好，想顺手总结一下今天的阅读吗？',
    '傍晚好，下班前有什么事需要我协助处理？',
    '傍晚好，今天有什么想进一步了解的话题吗？',
    '傍晚好，需要我帮你查点什么吗？',
  ],
  night: [
    '晚上好，有什么我可以帮你的吗？',
    '晚上好，今晚想聊点什么或查些什么？',
    '晚上好，需要我帮你整理今天的内容吗？',
    '晚上好，有什么想法想一起聊聊吗？',
    '晚上好，今晚准备读点什么呢？',
  ],
  lateNight: [
    '夜深了，还在忙碌吗？我一直都在。',
    '夜深了，有什么需要我帮你分担的吗？',
    '这么晚还没休息，是在查什么重要资料吗？',
    '夜深了，需要我帮你快速梳理一下手头的事吗？',
    '夜深人静，有什么灵感或问题想聊聊吗？',
  ],
}

export function timeSlotForHour(hour: number): TimeSlot {
  if (hour >= 5 && hour < 11) return 'morning'
  if (hour >= 11 && hour < 14) return 'noon'
  if (hour >= 14 && hour < 18) return 'afternoon'
  if (hour >= 18 && hour < 20) return 'evening'
  if (hour >= 20 && hour < 23) return 'night'
  return 'lateNight'
}

export function pickGreeting(now = new Date(), random = Math.random): string {
  const options = greetingsBySlot[timeSlotForHour(now.getHours())]
  const index = Math.min(options.length - 1, Math.floor(random() * options.length))
  return options[index]
}

/** 提取自 build/appicon.svg 的三层悬浮折页图形（不含外层圆角矩形底板）。 */
export function AppMark({ className }: { className?: string }) {
  const prefix = useId().replace(/:/g, '')
  const id = (name: string) => `${prefix}-${name}`

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="240 230 700 680"
      className={className}
      aria-hidden="true"
    >
      <defs>
        <filter id={id('markShadow')} x="-22%" y="-30%" width="150%" height="172%" colorInterpolationFilters="sRGB">
          <feGaussianBlur in="SourceAlpha" stdDeviation="15" result="blur" />
          <feOffset in="blur" dx="0" dy="17" result="offset" />
          <feFlood floodColor="#0A1838" floodOpacity=".12" result="tone" />
          <feComposite in="tone" in2="offset" operator="in" result="shadow" />
          <feMerge>
            <feMergeNode in="shadow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <filter id={id('upperLeafShadow')} x="-24%" y="-24%" width="150%" height="155%" colorInterpolationFilters="sRGB">
          <feGaussianBlur in="SourceAlpha" stdDeviation="6" result="blur" />
          <feOffset in="blur" dx="2" dy="6" result="offset" />
          <feFlood floodColor="#233A6E" floodOpacity=".18" result="tone" />
          <feComposite in="tone" in2="offset" operator="in" result="shadow" />
          <feMerge>
            <feMergeNode in="shadow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <linearGradient id={id('bottomLeaf')} x1="337" y1="665" x2="835" y2="869" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#102852" />
          <stop offset=".36" stopColor="#142E5E" />
          <stop offset=".75" stopColor="#24529C" />
          <stop offset="1" stopColor="#2E61AD" />
        </linearGradient>
        <radialGradient id={id('bottomSheen')} cx="0" cy="0" r="1" gradientTransform="translate(547 866) rotate(-68) scale(272 375)" gradientUnits="userSpaceOnUse">
          <stop stopColor="#2C61B3" stopOpacity=".52" />
          <stop offset="1" stopColor="#2C61B3" stopOpacity="0" />
        </radialGradient>
        <linearGradient id={id('middleLeaf')} x1="292" y1="698" x2="862" y2="481" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#416FAC" />
          <stop offset=".2" stopColor="#7398CD" />
          <stop offset=".44" stopColor="#A5C3F3" />
          <stop offset=".75" stopColor="#C7DBFF" />
          <stop offset="1" stopColor="#A8C4F4" />
        </linearGradient>
        <radialGradient id={id('middleSoftLight')} cx="0" cy="0" r="1" gradientTransform="translate(732 666) rotate(158) scale(390 210)" gradientUnits="userSpaceOnUse">
          <stop stopColor="#E3EEFF" stopOpacity=".65" />
          <stop offset="1" stopColor="#E3EEFF" stopOpacity="0" />
        </radialGradient>
        <linearGradient id={id('upperLeaf')} x1="465" y1="300" x2="616" y2="661" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#81A9E0" />
          <stop offset=".52" stopColor="#A2C2EF" />
          <stop offset=".85" stopColor="#B3CEF3" />
          <stop offset="1" stopColor="#A1BFE8" />
        </linearGradient>
        <linearGradient id={id('upperEdge')} x1="860" y1="255" x2="748" y2="450" gradientUnits="userSpaceOnUse">
          <stop stopColor="#D9DFED" stopOpacity=".72" />
          <stop offset=".35" stopColor="#CFD6E8" stopOpacity=".32" />
          <stop offset="1" stopColor="#7F98C4" stopOpacity=".08" />
        </linearGradient>
        <path id={id('bottomShape')} d="M268 754 C342 701 413 650 490 649 C579 647 650 671 712 698 C776 727 824 688 865 632 C895 644 906 659 900 683 C886 724 859 783 838 830 C820 877 785 889 742 880 C662 865 574 808 495 776 C415 743 338 741 268 757 Z" />
        <path id={id('middleShape')} d="M268 754 C294 671 370 605 486 565 C610 530 699 496 748 451 C787 466 849 486 892 499 C909 504 919 520 912 541 L867 632 C846 681 828 709 802 716 C766 729 726 718 682 705 C586 676 518 664 459 671 C381 677 311 717 268 754 Z" />
        <path id={id('topShape')} d="M268 753 C282 641 324 535 387 446 C477 322 652 281 868 254 C830 312 786 390 748 451 C716 504 680 529 600 551 C479 584 378 612 313 682 C291 706 276 729 268 753 Z" />
      </defs>
      <g filter={`url(#${id('markShadow')})`}>
        <use href={`#${id('bottomShape')}`} fill={`url(#${id('bottomLeaf')})`} />
        <use href={`#${id('bottomShape')}`} fill={`url(#${id('bottomSheen')})`} />
        <path d="M268 754 C349 696 422 650 491 649 C582 646 647 675 715 700" fill="none" stroke="#355B9B" strokeOpacity=".45" strokeWidth="2.4" />
        <use href={`#${id('middleShape')}`} fill={`url(#${id('middleLeaf')})`} />
        <use href={`#${id('middleShape')}`} fill={`url(#${id('middleSoftLight')})`} />
        <g filter={`url(#${id('upperLeafShadow')})`}>
          <use href={`#${id('topShape')}`} fill={`url(#${id('upperLeaf')})`} />
        </g>
        <path d="M868 254 C830 313 787 389 748 451" stroke={`url(#${id('upperEdge')})`} strokeWidth="1.25" fill="none" strokeLinecap="round" />
        <path d="M268 753 C282 641 324 535 387 446 C477 322 652 281 868 254" fill="none" stroke="#577FB6" strokeWidth="1.1" strokeOpacity=".72" />
      </g>
    </svg>
  )
}
