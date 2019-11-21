#!/bin/bash
#     Copyright 2019 Nexus Operator and/or its authors
#
#     This file is part of Nexus Operator.
#
#     Nexus Operator is free software: you can redistribute it and/or modify
#     it under the terms of the GNU General Public License as published by
#     the Free Software Foundation, either version 3 of the License, or
#     (at your option) any later version.
#
#     Nexus Operator is distributed in the hope that it will be useful,
#     but WITHOUT ANY WARRANTY; without even the implied warranty of
#     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#     GNU General Public License for more details.
#
#     You should have received a copy of the GNU General Public License
#     along with Nexus Operator.  If not, see <https://www.gnu.org/licenses/>.


if ! hash addlicense 2>/dev/null; then
  go get -u github.com/google/addlicense
fi

# https://github.com/google/addlicense
# https://www.gnu.org/licenses/gpl-howto.en.html

addlicense -c "Nexus Operator and/or its authors" -f LICENSE_NOTICE cmd hack pkg version tools.go