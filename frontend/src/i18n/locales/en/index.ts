import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import admin from './admin'
import misc from './misc'
import compat from './compat'
import local from './local'
import { mergeLocaleFallback } from '../merge'

const upstream = {
  ...landing,
  ...common,
  ...dashboard,
  admin,
  ...misc,
}

const current = mergeLocaleFallback(upstream, local)
export default mergeLocaleFallback(current, compat)
