import { useQuery } from '@tanstack/react-query'
import { navigationAPI,navigationQueryKey } from '@/lib/navigation-api'
export function useNavigation(){return useQuery({queryKey:navigationQueryKey,queryFn:navigationAPI.list,staleTime:30000,retry:false})}
