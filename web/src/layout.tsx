import { Link } from 'react-router-dom'

import { Box, Flex, Text } from '@radix-ui/themes'

import TopNavigation from './components/TopNavigation'

function Layout({
  logo,
  left,
  middle,
  right,
}: {
  logo: string
  left?: React.ReactNode
  middle?: React.ReactNode
  right?: React.ReactNode
}) {
  return (
    <Box className="w-screen h-[95vh] max-w-[95%] mx-auto flex flex-col">
      {/* Navbar, links are just a facade for now */}
      <Box className="w-full p-3 mb-1 border-b">
        <Flex align="center" justify="between">
          <Link to="/">
            <Flex align="center" gap="2">
              <img src={logo} className="h-6 w-6" />
              <Text size="5" weight="bold" className="text-black">
                Cloud Assistant
              </Text>
            </Flex>
          </Link>
          <Flex gap="4">
            <TopNavigation />
          </Flex>
        </Flex>
      </Box>

      {/* Main content */}
      <Flex className="w-full h-[95%] flex-1 gap-2">
        {/* Left */}
        <Box className="flex-2 flex flex-col h-full p-2 border-r border-gray-400">
          {left ?? <div />}
        </Box>

        {/* Middle */}
        <Box className="flex-3 flex flex-col h-full p-2 border-r border-gray-400">
          {middle ?? <div />}
        </Box>

        {/* Right */}
        <Box className="flex-1 flex flex-col h-full p-2">
          {right ?? <div />}
        </Box>
      </Flex>
    </Box>
  )
}

export default Layout
